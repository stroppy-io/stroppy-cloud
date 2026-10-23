// Claims of a GitHub sign-in → the identity's traits (methods.oidc, provider
// `github`). Kratos reads the primary e-mail from /user/emails (scope
// user:email), so a hidden profile address still arrives. `nickname` is the
// GitHub login, the name when the profile has none. The address counts as
// confirmed only when GitHub confirmed it.
local claims = std.extVar('claims');
local email = claims.email;
local name = if std.objectHas(claims, 'name') && claims.name != '' then claims.name else claims.nickname;
{
  identity: {
    traits: { email: email, name: std.substr(name, 0, 80) },
    verified_addresses: if std.objectHas(claims, 'email_verified') && claims.email_verified then [
      { via: 'email', value: email },
    ] else [],
  },
}
