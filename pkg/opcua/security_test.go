package opcua

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplySecurity(t *testing.T) {
	c := DefaultConfig()
	require.NoError(t, c.ApplySecurity("None", false))
	assert.True(t, c.AllowNone)
	assert.True(t, c.AllowAnonymous)

	c = DefaultConfig()
	require.NoError(t, c.ApplySecurity("basic256sha256", false))
	assert.False(t, c.AllowNone)
	assert.True(t, c.EnableBasic256Sha256)
	assert.False(t, c.AllowAnonymous)

	c = DefaultConfig()
	require.NoError(t, c.ApplySecurity("basic256sha256", true))
	assert.True(t, c.AllowAnonymous, "an explicit --allow-anonymous is kept")

	c = DefaultConfig()
	c.AllowAnonymous = false
	assert.ErrorContains(t, c.ApplySecurity("none", true), "needs --security basic256sha256")
	assert.ErrorContains(t, c.ApplySecurity("x", false), "invalid --security")
}

func TestExposedAndWarning(t *testing.T) {
	for addr, want := range map[string]bool{
		":4840": true, "0.0.0.0:4840": true, "[::]:4840": true, "192.168.1.5:4840": true, "plc.local:4840": true,
		"127.0.0.1:4840": false, "[::1]:4840": false, "localhost:4840": false, "bad": true,
	} {
		assert.Equal(t, want, Exposed(addr), addr)
	}
	c := DefaultConfig()
	assert.ErrorContains(t, AnonymousWriteWarning(c, ":4840"), "anonymous OPC UA clients can write on :4840")
	assert.NoError(t, AnonymousWriteWarning(c, "127.0.0.1:4840"))
	c.AllowAnonymousWrite = false
	assert.NoError(t, AnonymousWriteWarning(c, ":4840"))
	c.AllowAnonymousWrite, c.AllowAnonymous = true, false
	assert.NoError(t, AnonymousWriteWarning(c, ":4840"))
}
