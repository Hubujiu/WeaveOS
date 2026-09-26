package session_test

import (
 "net/http"
 "net/http/httptest"
 "testing"

 "github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

// Oracle: user Q7 and Redis Session §3/§6, re-read 2026-09-26.
func TestHostCookiesAndClearing(t *testing.T) {
 sid := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
 csrf := "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE"
 w := httptest.NewRecorder()
 if err := session.IssueCookies(w, sid, csrf); err != nil { t.Fatalf("known canonical tokens must issue two cookies: %v", err) }
 verify := func(cookies []*http.Cookie, maxAge int) {
  t.Helper()
  if len(cookies)!=2 { t.Fatalf("want two separate cookies, got %d",len(cookies)) }
  expected := map[string]bool{"__Host-session":true,"__Host-csrf":false}
  for _, c := range cookies {
   httpOnly,ok:=expected[c.Name]
   if !ok || c.HttpOnly!=httpOnly || !c.Secure || c.SameSite!=http.SameSiteLaxMode || c.Path!="/" || c.Domain!="" || c.MaxAge!=maxAge {t.Errorf("Cookie security attributes differ for %s",c.Name)}
   delete(expected,c.Name)
  }
  if len(expected)!=0 {t.Error("required Cookie name absent")}
 }
 cookies:=w.Result().Cookies(); verify(cookies,3600)
 if cookies[0].Value!=sid || cookies[1].Value!=csrf {t.Error("independent Cookie values differ")}
 cleared:=httptest.NewRecorder(); session.ClearCookies(cleared); verify(cleared.Result().Cookies(),-1)
 for _,c:=range cleared.Result().Cookies(){if c.Value!=""{t.Error("cleared Cookie still contains a value")}}
}
