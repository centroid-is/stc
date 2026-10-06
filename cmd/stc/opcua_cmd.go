package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/opcua/opcuatest"
	"github.com/spf13/cobra"
)

// snapshotClientURI is the ApplicationURI of the auto-generated client cert.
const snapshotClientURI = "urn:stc:opcua-snapshot"

func newOpcuaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opcua",
		Short: "OPC UA client tools",
	}
	cmd.AddCommand(newOpcuaSnapshotCmd())
	return cmd
}

func newOpcuaSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot <endpoint>",
		Short: "Capture the browse snapshot of an OPC UA server (golden schema)",
		Long: `Connect to an OPC UA server (for example a TwinCAT TF6100 at
opc.tcp://<plc>:4840), browse namespace 4 from --root and write the node
classes, data types, value ranks, array dimensions, access levels and
DataType definitions as JSON. The schema equals tests/opcua_golden/st301_shape.json,
so the file can be diffed against stc serve.

The command only browses and reads attributes. It never writes values or
calls methods, and the snapshot holds no values.`,
		Args: cobra.ExactArgs(1),
		RunE: runOpcuaSnapshot,
	}
	cmd.Flags().StringP("out", "o", "", "Write the snapshot to this file (default: stdout)")
	cmd.Flags().String("root", "ns=4;s=PLC1", "NodeId to browse from")
	cmd.Flags().String("security", "none", "Security mode: none (SecurityPolicy None + Anonymous) or basic256sha256 (SignAndEncrypt)")
	cmd.Flags().String("cert", "", "Client certificate for basic256sha256 (generated when empty)")
	cmd.Flags().String("key", "", "Client private key; required with --cert")
	cmd.Flags().String("pki-dir", "", "Directory for the generated client certificate (default: user cache dir)")
	cmd.Flags().Duration("timeout", 10*time.Second, "Timeout for connecting and browsing")
	return cmd
}

// snapshotStatus is the --format json result.
type snapshotStatus struct {
	Out       string `json:"out"`
	Nodes     int    `json:"nodes"`
	DataTypes int    `json:"data_types"`
}

func runOpcuaSnapshot(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	endpoint := args[0]
	format, _ := cmd.Flags().GetString("format")
	outPath, _ := cmd.Flags().GetString("out")
	rootText, _ := cmd.Flags().GetString("root")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	if timeout <= 0 {
		return fmt.Errorf("--timeout must be positive, got %s", timeout)
	}
	root := ua.ParseNodeID(rootText)
	if root == nil {
		return fmt.Errorf("invalid --root %q: want a NodeId such as ns=4;s=PLC1", rootText)
	}
	opts, certID, err := snapshotClientOptions(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	c, err := client.Dial(ctx, endpoint, opts...)
	if err != nil && certID != nil && identityRejected(err) {
		// A secure server that refuses Anonymous (stc serve
		// --security basic256sha256) takes the client certificate as the
		// user identity.
		c, err = client.Dial(ctx, endpoint, append(opts, certID)...)
	}
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", endpoint, err)
	}
	defer func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer ccancel()
		if err := c.Close(cctx); err != nil {
			_ = c.Abort(cctx)
		}
	}()

	snap, err := opcuatest.Take(ctx, c, root)
	if err != nil {
		return fmt.Errorf("browsing %s from %s: %w", endpoint, rootText, err)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	out := cmd.OutOrStdout()
	if outPath == "" {
		// The snapshot is itself JSON, so --format json prints it unchanged.
		_, err := out.Write(data)
		return err
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return err
	}
	if format == "json" {
		b, _ := json.Marshal(snapshotStatus{Out: outPath, Nodes: len(snap.Nodes), DataTypes: len(snap.DataTypes)})
		fmt.Fprintln(out, string(b))
		return nil
	}
	fmt.Fprintf(out, "wrote %s: %d nodes, %d data types\n", outPath, len(snap.Nodes), len(snap.DataTypes))
	return nil
}

// identityRejected reports whether a dial failed on the user identity.
func identityRejected(err error) bool {
	return errors.Is(err, ua.BadIdentityTokenRejected) || errors.Is(err, ua.BadIdentityTokenInvalid) ||
		errors.Is(err, ua.BadUserAccessDenied)
}

// snapshotClientOptions maps --security, --cert, --key and --pki-dir to
// awcullen client options. The server certificate is not verified: this
// is a development tool (T-29-07). With basic256sha256 it also returns the
// option presenting the client certificate as the user identity, used when
// the server rejects Anonymous.
func snapshotClientOptions(cmd *cobra.Command) ([]client.Option, client.Option, error) {
	opts := []client.Option{client.WithInsecureSkipVerify()}
	sec, _ := cmd.Flags().GetString("security")
	switch strings.ToLower(sec) {
	case "none":
		return append(opts, client.WithSecurityPolicyURI(ua.SecurityPolicyURINone, ua.MessageSecurityModeNone)), nil, nil
	case "basic256sha256":
	default:
		return nil, nil, fmt.Errorf("invalid --security %q: want none or basic256sha256", sec)
	}
	certPath, _ := cmd.Flags().GetString("cert")
	keyPath, _ := cmd.Flags().GetString("key")
	if (certPath == "") != (keyPath == "") {
		return nil, nil, errors.New("--cert and --key must be given together")
	}
	if certPath == "" {
		dir, _ := cmd.Flags().GetString("pki-dir")
		if dir == "" {
			cache, err := os.UserCacheDir()
			if err != nil {
				return nil, nil, fmt.Errorf("no --pki-dir and no user cache dir: %w", err)
			}
			dir = filepath.Join(cache, "stc", "opcua-client-pki")
		}
		var err error
		certPath, keyPath, err = opcua.EnsureCert(dir, snapshotClientURI)
		if err != nil {
			return nil, nil, fmt.Errorf("client certificate: %w", err)
		}
	}
	return append(opts,
		client.WithSecurityPolicyURI(ua.SecurityPolicyURIBasic256Sha256, ua.MessageSecurityModeSignAndEncrypt),
		client.WithClientCertificatePaths(certPath, keyPath)), client.WithX509IdentityPaths(certPath, keyPath), nil
}
