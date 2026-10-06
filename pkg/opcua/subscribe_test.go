package opcua

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
)

// subDeadline bounds every wait for a notification (D-05).
const subDeadline = 5 * time.Second

// quietWindow is how long a constant value must produce no data change.
const quietWindow = 500 * time.Millisecond

// subscriber runs a client Publish loop and collects data-change
// notifications per client handle. Keep-alives carry no notification data
// and are not recorded.
type subscriber struct {
	mu   sync.Mutex
	got  map[uint32][]ua.DataValue
	wake chan struct{}
	done chan struct{}
}

func subscribe(t *testing.T, c *client.Client, items []ua.MonitoredItemCreateRequest) (*subscriber, []ua.MonitoredItemCreateResult) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), subDeadline)
	defer cancel()
	sub, err := c.CreateSubscription(ctx, &ua.CreateSubscriptionRequest{
		RequestedPublishingInterval: 100, RequestedMaxKeepAliveCount: 30,
		RequestedLifetimeCount: 120, PublishingEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}
	res, err := c.CreateMonitoredItems(ctx, &ua.CreateMonitoredItemsRequest{
		SubscriptionID: sub.SubscriptionID, TimestampsToReturn: ua.TimestampsToReturnBoth,
		ItemsToCreate: items,
	})
	if err != nil {
		t.Fatalf("CreateMonitoredItems: %v", err)
	}
	s := &subscriber{got: map[uint32][]ua.DataValue{}, wake: make(chan struct{}, 1), done: make(chan struct{})}
	loopCtx, stop := context.WithCancel(context.Background())
	go s.loop(loopCtx, c)
	t.Cleanup(func() {
		stop()
		<-s.done
	})
	return s, res.Results
}

func (s *subscriber) loop(ctx context.Context, c *client.Client) {
	defer close(s.done)
	var acks []ua.SubscriptionAcknowledgement
	for ctx.Err() == nil {
		res, err := c.Publish(ctx, &ua.PublishRequest{SubscriptionAcknowledgements: acks})
		if err != nil {
			return
		}
		acks = []ua.SubscriptionAcknowledgement{{SubscriptionID: res.SubscriptionID, SequenceNumber: res.NotificationMessage.SequenceNumber}}
		if len(res.NotificationMessage.NotificationData) == 0 {
			acks = nil // keep-alive: nothing to acknowledge
			continue
		}
		s.mu.Lock()
		for _, nd := range res.NotificationMessage.NotificationData {
			var dcn ua.DataChangeNotification
			switch n := nd.(type) {
			case ua.DataChangeNotification:
				dcn = n
			case *ua.DataChangeNotification:
				dcn = *n
			default:
				continue
			}
			for _, mi := range dcn.MonitoredItems {
				s.got[mi.ClientHandle] = append(s.got[mi.ClientHandle], mi.Value)
			}
		}
		s.mu.Unlock()
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// count returns the number of notifications received for handle.
func (s *subscriber) count(handle uint32) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.got[handle])
}

// value returns the n-th notification for handle.
func (s *subscriber) value(handle uint32, n int) ua.DataValue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.got[handle][n]
}

// waitCount waits until handle has at least n notifications.
func (s *subscriber) waitCount(t *testing.T, handle uint32, n int) {
	t.Helper()
	deadline := time.After(subDeadline)
	for s.count(handle) < n {
		select {
		case <-s.wake:
		case <-deadline:
			t.Fatalf("handle %d: %d notifications after %s, want %d", handle, s.count(handle), subDeadline, n)
		}
	}
}

func monitor(id ua.NodeID, handle uint32) ua.MonitoredItemCreateRequest {
	return ua.MonitoredItemCreateRequest{
		ItemToMonitor:  ua.ReadValueID{NodeID: id, AttributeID: ua.AttributeIDValue},
		MonitoringMode: ua.MonitoringModeReporting,
		RequestedParameters: ua.MonitoringParameters{
			ClientHandle: handle, SamplingInterval: 50, QueueSize: 10, DiscardOldest: true,
		},
	}
}

// TestSubscriptionSamplesSource proves OPCUA-08 (D-04, D-05): monitored
// items sample the NodeSource through the read handlers, so a value changed
// in the source reaches a subscribed client without any push, and a
// constant value produces no data change.
func TestSubscriptionSamplesSource(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	sp, vals := testSpace()
	src := NewMapSource(vals)
	if err := s.Publish(sp, src); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c := dialAnon(t, s)
	hmi := "GVL_Test.fbMotor.HMI"
	const (
		hBool    = 1
		hStruct  = 2
		hUnknown = 3
	)
	sub, results := subscribe(t, c, []ua.MonitoredItemCreateRequest{
		monitor(ua.NewNodeIDString(4, hmi+".p_cmd_JogFwd"), hBool),
		monitor(ua.NewNodeIDString(4, hmi), hStruct),
		monitor(ua.NewNodeIDString(4, "GVL_Test.nope"), hUnknown),
	})
	if len(results) != 3 {
		t.Fatalf("results = %d", len(results))
	}
	for i, r := range results[:2] {
		if r.StatusCode != ua.Good {
			t.Fatalf("item %d status %v", i, r.StatusCode)
		}
	}
	if results[2].StatusCode != ua.BadNodeIDUnknown {
		t.Errorf("unknown node status = %v, want BadNodeIdUnknown", results[2].StatusCode)
	}

	// Initial values.
	sub.waitCount(t, hBool, 1)
	sub.waitCount(t, hStruct, 1)
	if v := sub.value(hBool, 0); v.StatusCode != ua.Good || v.Value != false {
		t.Fatalf("initial bool = %v %v", v.Value, v.StatusCode)
	}
	if v := sub.value(hStruct, 0); v.StatusCode != ua.Good || v.Value == nil {
		t.Fatalf("initial struct = %v %v", v.Value, v.StatusCode)
	}

	// Constant values: no data change, only keep-alives.
	time.Sleep(quietWindow)
	if n, m := sub.count(hBool), sub.count(hStruct); n != 1 || m != 1 {
		t.Fatalf("notifications while constant: bool %d, struct %d", n, m)
	}

	// One change in the source: exactly one data change per item.
	src.Set(hmi+".p_cmd_JogFwd", true)
	sub.waitCount(t, hBool, 2)
	sub.waitCount(t, hStruct, 2)
	if v := sub.value(hBool, 1); v.StatusCode != ua.Good || v.Value != true {
		t.Fatalf("changed bool = %v %v", v.Value, v.StatusCode)
	}
	time.Sleep(quietWindow)
	if n, m := sub.count(hBool), sub.count(hStruct); n != 2 || m != 2 {
		t.Fatalf("after one change: bool %d, struct %d notifications, want 2 and 2", n, m)
	}
	if n := sub.count(hUnknown); n != 0 {
		t.Errorf("unknown item delivered %d notifications", n)
	}
}
