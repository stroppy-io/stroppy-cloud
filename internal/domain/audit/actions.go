package audit

// Actions is every action the server records (the contract's AuditAction is
// the same list; a test keeps the code's Action literals inside it).
var Actions = []string{
	"admin.pipelines.resync", "admin.run.cancel", "admin.settings.update",
	"admin.tenant.assign_owner", "admin.tenant.delete", "admin.tenant.resume", "admin.tenant.suspend",
	"admin.user.platform_admin",
	"database.create", "database.delete", "database.update",
	"invite.accept", "invite.create", "invite.revoke",
	"member.leave", "member.remove", "member.role",
	"provider.create", "provider.delete", "provider.update",
	"run.cancel", "run.delete", "run.keep.extend", "run.keep.release", "run.launch", "run.update",
	"schedule.create", "schedule.delete", "schedule.update",
	"share.create", "share.rebuild", "share.revoke",
	"suite.create", "suite.delete", "suite.launch", "suite.update",
	"suite_run.cancel", "suite_run.delete",
	"tenant.create", "tenant.delete", "tenant.transfer", "tenant.update",
	"test.create", "test.delete", "test.update",
	"token.create", "token.revoke",
	"webhook.create", "webhook.delete", "webhook.rotate", "webhook.update",
	"workload.create", "workload.delete", "workload.update",
}

// TargetKinds is every kind of object an entry is about (AuditTargetKind).
var TargetKinds = []string{
	"database", "invite", "namespace", "provider", "run", "schedule", "share", "suite", "suite_run",
	"system_settings", "tenant", "test", "token", "user", "webhook", "workload",
}
