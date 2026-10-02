// Claims of the session JWT returned by `GET /sessions/whoami?tokenize_as=stroppy`.
// The tokenized JWT is the ONLY thing the stroppy server consumes: it verifies
// the signature locally against kratos-public-jwks.json and reads the actor
// from these claims (sub = Kratos identity id, sid = session id).
//
// email_verified is derived from the verifiable address matching the trait:
// the server matches tenant invites ONLY against verified e-mails.
local claims = std.extVar('claims');
local session = std.extVar('session');
local traits = session.identity.traits;
local addrs = std.filter(
  function(a) std.objectHas(a, 'value') && a.value == traits.email,
  session.identity.verifiable_addresses
);
local verified = if std.length(addrs) > 0 && std.objectHas(addrs[0], 'verified') && addrs[0].verified then true else false;

{
  claims: {
    iss: 'stroppy-cloud',
    aud: 'stroppy-cloud',
    email: traits.email,
    email_verified: verified,
    name: if std.objectHas(traits, 'name') then traits.name else '',
    aal: session.authenticator_assurance_level
  }
}
