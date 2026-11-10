package provision

import (
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestCloudRefusalOnlyForGood(t *testing.T) {
	synced := func(status corev1.ConditionStatus, reason xpv1.ConditionReason, msg string) xpv1.Condition {
		return xpv1.Condition{Type: xpv1.TypeSynced, Status: status, Reason: reason, Message: msg}
	}
	for _, tc := range []struct {
		name string
		c    xpv1.Condition
		fail bool
	}{
		{"yandex quota", synced(corev1.ConditionFalse, xpv1.ReasonReconcileError, "create failed: rpc error: code = ResourceExhausted desc = Quota limit compute.ssdDisks.size exceeded"), true},
		{"yandex rights", synced(corev1.ConditionFalse, xpv1.ReasonReconcileError, "rpc error: code = PermissionDenied desc = Permission denied"), true},
		{"aws vcpu limit", synced(corev1.ConditionFalse, xpv1.ReasonReconcileError, "api error VcpuLimitExceeded: You have requested more vCPU capacity"), true},
		{"aws capacity recovers", synced(corev1.ConditionFalse, xpv1.ReasonReconcileError, "api error InsufficientInstanceCapacity"), false},
		{"transient", synced(corev1.ConditionFalse, xpv1.ReasonReconcileError, "connection reset by peer"), false},
		// The API's request rate, not a resource quota: the controller retries it.
		{"yandex request rate", synced(corev1.ConditionFalse, xpv1.ReasonReconcileError, "create failed: apply failed: Error while waiting operation to create instance: operation (id=fv41q0ho4ppq3e58dctu) poll fail: rpc error: code = ResourceExhausted desc = Request quota exceeded: "), false},
		{"aws request rate", synced(corev1.ConditionFalse, xpv1.ReasonReconcileError, "api error RequestLimitExceeded: Request limit exceeded."), false},
		{"aws throttling", synced(corev1.ConditionFalse, xpv1.ReasonReconcileError, "api error Throttling: Rate exceeded"), false},
		{"synced", synced(corev1.ConditionTrue, xpv1.ReasonReconcileSuccess, ""), false},
	} {
		if got := cloudRefusal(tc.c) != nil; got != tc.fail {
			t.Errorf("%s: refusal %v, want %v", tc.name, got, tc.fail)
		}
	}
}
