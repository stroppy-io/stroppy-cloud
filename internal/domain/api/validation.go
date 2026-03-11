package api

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	"github.com/stroppy-io/hatchet-workflow/internal/domain/topology"
	pb "github.com/stroppy-io/hatchet-workflow/internal/proto/api"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
)

func (h *Handler) ValidateTopology(
	ctx context.Context,
	req *connect.Request[pb.ValidateTopologyRequest],
) (*connect.Response[pb.ValidateTopologyResponse], error) {
	test := req.Msg.GetTest()
	if test == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("test is required"))
	}

	tmpl := test.GetDatabaseTemplate()
	if tmpl == nil {
		// connection_string mode — no topology to validate.
		if test.GetConnectionString() != "" {
			return connect.NewResponse(&pb.ValidateTopologyResponse{Valid: true}), nil
		}
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("test must have either database_template or connection_string"))
	}

	err := topology.ValidateDatabaseTemplate(ctx, tmpl)
	if err == nil {
		return connect.NewResponse(&pb.ValidateTopologyResponse{Valid: true}), nil
	}

	return connect.NewResponse(&pb.ValidateTopologyResponse{
		Valid: false,
		Issues: []*pb.ValidationIssue{{
			Severity: pb.ValidationSeverity_VALIDATION_SEVERITY_ERROR,
			Field:    "database_template",
			Message:  err.Error(),
		}},
	}), nil
}

func (h *Handler) DryRun(
	ctx context.Context,
	req *connect.Request[pb.DryRunRequest],
	stream *connect.ServerStream[pb.DryRunCheck],
) error {
	suite := req.Msg.GetSuite()
	settings := req.Msg.GetSettings()
	target := req.Msg.GetTarget()

	// Check: settings present.
	if err := sendCheck(stream, "settings_present", func() (pb.DryRunCheckStatus, string, string) {
		if settings == nil {
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED, "settings are required", ""
		}
		return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_PASSED, "settings present", ""
	}); err != nil {
		return err
	}

	// Check: suite present and has tests.
	if err := sendCheck(stream, "suite_valid", func() (pb.DryRunCheckStatus, string, string) {
		if suite == nil {
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED, "test suite is required", ""
		}
		if len(suite.GetTests()) == 0 {
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED, "test suite has no tests", ""
		}
		return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_PASSED,
			fmt.Sprintf("suite has %d test(s)", len(suite.GetTests())), ""
	}); err != nil {
		return err
	}

	// Check: target-specific settings.
	if err := sendCheck(stream, "target_settings", func() (pb.DryRunCheckStatus, string, string) {
		if settings == nil {
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_SKIPPED, "no settings to check", ""
		}
		switch target {
		case deployment.Target_TARGET_YANDEX_CLOUD:
			yc := settings.GetYandexCloud()
			if yc == nil {
				return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED,
					"yandex_cloud settings are required for TARGET_YANDEX_CLOUD", ""
			}
			if yc.GetProviderSettings() == nil {
				return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED,
					"yandex_cloud.provider_settings is required", ""
			}
			if yc.GetNetworkSettings() == nil {
				return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED,
					"yandex_cloud.network_settings is required", ""
			}
			if yc.GetVmSettings() == nil {
				return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED,
					"yandex_cloud.vm_settings is required", ""
			}
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_PASSED, "yandex cloud settings valid", ""
		case deployment.Target_TARGET_DOCKER:
			if settings.GetDocker() == nil {
				return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED,
					"docker settings are required for TARGET_DOCKER", ""
			}
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_PASSED, "docker settings valid", ""
		default:
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED,
				fmt.Sprintf("unknown target: %s", target), ""
		}
	}); err != nil {
		return err
	}

	// Check: hatchet connection.
	if err := sendCheck(stream, "hatchet_connection", func() (pb.DryRunCheckStatus, string, string) {
		if settings == nil {
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_SKIPPED, "no settings to check", ""
		}
		hc := settings.GetHatchetConnection()
		if hc == nil {
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED, "hatchet_connection is required", ""
		}
		if hc.GetToken() == "" {
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED, "hatchet_connection.token is empty", ""
		}
		if hc.GetHost() == "" {
			return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED, "hatchet_connection.host is empty", ""
		}
		return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_PASSED, "hatchet connection configured", ""
	}); err != nil {
		return err
	}

	// Check: validate topology for each test.
	if suite != nil {
		for i, test := range suite.GetTests() {
			checkName := fmt.Sprintf("topology_test_%d", i)
			if name := test.GetName(); name != "" {
				checkName = fmt.Sprintf("topology_%s", name)
			}
			if err := sendCheck(stream, checkName, func() (pb.DryRunCheckStatus, string, string) {
				tmpl := test.GetDatabaseTemplate()
				if tmpl == nil {
					if test.GetConnectionString() != "" {
						return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_PASSED,
							"using external connection string", ""
					}
					return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED,
						"test must have database_template or connection_string", ""
				}
				if err := topology.ValidateDatabaseTemplate(ctx, tmpl); err != nil {
					return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_FAILED,
						"topology validation failed", err.Error()
				}

				ipCount := topology.RequiredIPCount(tmpl)
				return pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_PASSED,
					fmt.Sprintf("topology valid, requires %d IP(s)", ipCount), ""
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// sendCheck streams a RUNNING → result pair for a single dry-run check.
func sendCheck(
	stream *connect.ServerStream[pb.DryRunCheck],
	name string,
	fn func() (pb.DryRunCheckStatus, string, string),
) error {
	_ = stream.Send(&pb.DryRunCheck{
		CheckName: name,
		Status:    pb.DryRunCheckStatus_DRY_RUN_CHECK_STATUS_RUNNING,
		Message:   "checking...",
	})

	status, msg, detail := fn()
	check := &pb.DryRunCheck{
		CheckName: name,
		Status:    status,
		Message:   msg,
	}
	if detail != "" {
		check.Detail = &detail
	}
	return stream.Send(check)
}
