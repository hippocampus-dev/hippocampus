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

local function escape_html(value)
  return (value:gsub("[&<>\"']", { ["&"] = "&amp;", ["<"] = "&lt;", [">"] = "&gt;", ['"'] = "&quot;", ["'"] = "&#39;" }))
end

function _M.login()
  local method = ngx.req.get_method()
  local arguments
  if method == "POST" then
    ngx.req.read_body()
    arguments = ngx.req.get_post_args()
  else
    arguments = ngx.req.get_uri_args()
  end
  local redirect_uri = arguments.redirect_uri
  local state = arguments.state
  if type(redirect_uri) ~= "string" or type(state) ~= "string" then
    return ngx.exit(ngx.HTTP_BAD_REQUEST)
  end
  -- https://www.rfc-editor.org/rfc/rfc8252#section-7.3
  local loopback = redirect_uri:match("^http://127%.0%.0%.1:%d+$") ~= nil
  -- https://www.rfc-editor.org/rfc/rfc8252#section-7.2, verified against /.well-known/assetlinks.json
  if not loopback and redirect_uri ~= "https://" .. ngx.var.http_host .. "/taurim/callback" then
    return ngx.exit(ngx.HTTP_BAD_REQUEST)
  end
  -- Any Android app can listen on 127.0.0.1: https://www.rfc-editor.org/rfc/rfc8252#section-8.1
  -- Chrome's desktop site mode sends a desktop User-Agent: https://developer.chrome.com/blog/desktop-mode
  if loopback and (ngx.var.http_user_agent or ""):find("Android", 1, true) then
    return ngx.exit(ngx.HTTP_FORBIDDEN)
  end

  -- Any local process can open this URL
  if method ~= "POST" then
    _M.authenticate()
    ngx.header["Content-Type"] = "text/html; charset=utf-8"
    ngx.header["Content-Security-Policy"] = "frame-ancestors 'none'"
    ngx.say(string.format(
      [[<!doctype html><meta name="viewport" content="width=device-width"><title>tauri-releases</title><form method="post" action="/login"><p>Sign in the application at %s? Continue only if you started signing in from taurin on a computer or from taurim.</p><input type="hidden" name="redirect_uri" value="%s"><input type="hidden" name="state" value="%s"><button type="submit">Sign in</button></form>]],
      escape_html(redirect_uri),
      escape_html(redirect_uri),
      escape_html(state)
    ))
    return ngx.exit(ngx.HTTP_OK)
  end

  if (ngx.var.http_origin or ""):match("^https?://([^/]+)$") ~= ngx.var.http_host then
    return ngx.exit(ngx.HTTP_FORBIDDEN)
  end
  _M.authenticate("deny")
  return ngx.redirect(redirect_uri .. "?" .. ngx.encode_args({ token = ngx.var.cookie_session, state = state }), ngx.HTTP_SEE_OTHER)
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
