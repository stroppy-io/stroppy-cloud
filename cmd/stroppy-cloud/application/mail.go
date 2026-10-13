package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/mail"
)

// inviteMailer renders the invite e-mail: plain text, one link.
type inviteMailer struct {
	sender    mail.Sender
	publicURL string
	log       *xlog.Logger
}

func (m inviteMailer) SendInvite(ctx context.Context, to string, t tenant.Tenant, inv tenant.Invite) error {
	text := fmt.Sprintf(`You are invited to join the tenant "%s" on Stroppy Cloud as %s.

Sign in (or sign up) with this e-mail address and confirm it — the invite
applies on its own; it is also listed here:
%s/me/invites

`, t.Name, inv.Role, strings.TrimRight(m.publicURL, "/"))
	if inv.Message != "" {
		text += "Message from the inviter:\n" + inv.Message + "\n\n"
	}
	text += fmt.Sprintf("The invite expires on %s.\n", inv.ExpiresAt.UTC().Format("2006-01-02 15:04 UTC"))
	err := m.sender.Send(ctx, mail.Message{To: to, Subject: fmt.Sprintf("Invitation to %s on Stroppy Cloud", t.Name), Text: text})
	if err != nil {
		m.log.Ctx().Warn(ctx, "invite mail failed", xlog.Email("to", to), xlog.ErrorCause(err))
	}
	return err
}
