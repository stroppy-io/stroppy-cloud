package cloud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/quotamanager/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type quotaListFunc func(*quotamanager.ListQuotaLimitsRequest) (*quotamanager.ListQuotaLimitsResponse, error)

func (f quotaListFunc) List(_ context.Context, req *quotamanager.ListQuotaLimitsRequest, _ ...grpc.CallOption) (*quotamanager.ListQuotaLimitsResponse, error) {
	return f(req)
}

func TestYandexQuotasPaginationAndScope(t *testing.T) {
	var calls []string
	client := quotaListFunc(func(req *quotamanager.ListQuotaLimitsRequest) (*quotamanager.ListQuotaLimitsResponse, error) {
		require.Equal(t, "resource-manager.cloud", req.Resource.Type)
		require.Equal(t, "cloud-id", req.Resource.Id)
		calls = append(calls, req.Service+":"+req.PageToken)
		switch req.Service + ":" + req.PageToken {
		case "compute:":
			return &quotamanager.ListQuotaLimitsResponse{NextPageToken: "page2", QuotaLimits: []*quotamanager.QuotaLimit{{QuotaId: "unrelated"}}}, nil
		case "compute:page2":
			return &quotamanager.ListQuotaLimitsResponse{QuotaLimits: []*quotamanager.QuotaLimit{{QuotaId: "compute.instanceCores.count", Limit: wrapperspb.Double(32), Usage: wrapperspb.Double(8)}}}, nil
		default:
			return &quotamanager.ListQuotaLimitsResponse{}, nil
		}
	})
	res, err := readYandexQuotas(context.Background(), client, "cloud-id")
	require.NoError(t, err)
	require.Equal(t, []string{"compute:", "compute:page2", "vpc:"}, calls)
	require.Equal(t, "cloud:cloud-id", res.Scope)
	require.Empty(t, res.UnavailableReason)
	require.Len(t, res.Quotas, 1)
	require.Equal(t, float64(32), res.Quotas[0].Limit)
	require.Equal(t, float64(8), res.Quotas[0].Used)
	require.Empty(t, res.Quotas[0].Zone, "cloud quotas must not be labeled as zone quotas")
}

func TestYandexQuotasPermissionDenied(t *testing.T) {
	for _, deniedCall := range []int{1, 2, 3} {
		t.Run(string(rune('0'+deniedCall)), func(t *testing.T) {
			calls := 0
			client := quotaListFunc(func(req *quotamanager.ListQuotaLimitsRequest) (*quotamanager.ListQuotaLimitsResponse, error) {
				calls++
				if calls == deniedCall {
					return nil, status.Error(codes.PermissionDenied, "not authorized")
				}
				page := &quotamanager.ListQuotaLimitsResponse{QuotaLimits: []*quotamanager.QuotaLimit{{QuotaId: "compute.instanceCores.count", Limit: wrapperspb.Double(32), Usage: wrapperspb.Double(8)}}}
				if req.PageToken == "" && req.Service == "compute" {
					page.NextPageToken = "page2"
				}
				return page, nil
			})
			res, err := readYandexQuotas(context.Background(), client, "cloud-id")
			require.NoError(t, err)
			require.Equal(t, deniedCall, calls)
			require.Equal(t, "permission_denied", res.UnavailableReason)
			require.Equal(t, "cloud:cloud-id", res.Scope)
			require.False(t, res.ObservedAt.IsZero())
			require.NotNil(t, res.Quotas)
			require.Empty(t, res.Quotas, "partial snapshots must be discarded")
		})
	}
}

func TestYandexQuotasOperationalErrorsRemainErrors(t *testing.T) {
	for _, code := range []codes.Code{codes.Unauthenticated, codes.Unavailable, codes.DeadlineExceeded, codes.Canceled, codes.InvalidArgument} {
		t.Run(code.String(), func(t *testing.T) {
			client := quotaListFunc(func(*quotamanager.ListQuotaLimitsRequest) (*quotamanager.ListQuotaLimitsResponse, error) {
				return nil, status.Error(code, "request failed")
			})
			res, err := readYandexQuotas(context.Background(), client, "cloud-id")
			require.Error(t, err)
			require.Equal(t, code, status.Code(err))
			require.Empty(t, res.UnavailableReason)
		})
	}
}
