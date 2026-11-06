local openidc = require("resty.openidc")

local _M = {}

local allowed_emails = {}
for email in string.gmatch(os.getenv("ALLOWED_EMAILS") or "", "[^,]+") do
  allowed_emails[email] = true
end

local options = {
  discovery = "https://accounts.google.com/.well-known/openid-configuration",
  client_id = os.getenv("CLIENT_ID"),
  client_secret = os.getenv("CLIENT_SECRET"),
  redirect_uri = os.getenv("REDIRECT_URI"),
  scope = "openid email",
  use_pkce = true,
  session_contents = { id_token = true },
}

-- tauri-plugin-updater keeps no Set-Cookie, so the cookie handed to the application must stay valid without renewal
local session_options = {
  secret = os.getenv("SESSION_SECRET"),
  idling_timeout = 0,
  rolling_timeout = 0,
  absolute_timeout = 0,
}

function _M.authenticate(unauth_action)
  local response, err = openidc.authenticate(options, nil, unauth_action, session_options)
  if err then
    ngx.log(ngx.ERR, err)
    return ngx.exit(ngx.HTTP_UNAUTHORIZED)
  end
  if response and (response.id_token.email_verified ~= true or not allowed_emails[response.id_token.email]) then
    return ngx.exit(ngx.HTTP_FORBIDDEN)
  end
end

function _M.login()
  local arguments = ngx.req.get_uri_args()
  local redirect_uri = arguments.redirect_uri
  local state = arguments.state
  -- https://www.rfc-editor.org/rfc/rfc8252#section-7.3
  if type(redirect_uri) ~= "string" or not redirect_uri:match("^http://127%.0%.0%.1:%d+$") or type(state) ~= "string" then
    return ngx.exit(ngx.HTTP_BAD_REQUEST)
  end

  _M.authenticate()
  return ngx.redirect(redirect_uri .. "?" .. ngx.encode_args({ token = ngx.var.cookie_session, state = state }))
end

function _M.verify()
  local token = (ngx.var.http_authorization or ""):match("^Bearer (.+)$")
  if not token then
    return ngx.exit(ngx.HTTP_UNAUTHORIZED)
  end

  ngx.req.set_header("Cookie", "session=" .. token)
  _M.authenticate("deny")
end

return _M
