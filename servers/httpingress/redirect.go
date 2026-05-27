package httpingress

// safeReturnPath constrains the post-auth redirect target to local same-host
// paths. state.ReturnPath is user-controllable (it's whatever path the user was
// originally trying to reach when auth kicked in), so feeding it straight into
// http.Redirect lets a crafted "//evil.example" bounce the user off-host on a
// successful login. Browsers also treat "/\evil" as a protocol-relative URL on
// some platforms. We require a leading "/" and reject "//..." or "/\..."
// second-character escapes; anything else falls back to "/".
func safeReturnPath(p string) string {
	if p == "" || p[0] != '/' {
		return "/"
	}
	if len(p) > 1 && (p[1] == '/' || p[1] == '\\') {
		return "/"
	}
	return p
}
