package security_test

import (
 "net/http"
 "net/http/httptest"
 "testing"
 "github.com/Hubujiu/WeaveOS/services/bff/internal/security"
)

func TestSourceMustMatchConfiguredOrigin(t *testing.T) {
 for _,tc:=range []struct{origin,referer string;want bool}{
  {"https://weaveos.test","",true},{"","https://weaveos.test/register?x=1",true},
  {"https://evil.test","https://weaveos.test/",false},{"null","",false},{"","",false},
  {"https://weaveos.test.evil.test","",false},{"","https://weaveos.test@evil.test/",false},
  {"http://weaveos.test","",false},{"https://weaveos.test:444","",false},
 } {
  r:=httptest.NewRequest("POST","https://weaveos.test/api/v1/sessions",nil)
  if tc.origin!=""{r.Header.Set("Origin",tc.origin)}; if tc.referer!=""{r.Header.Set("Referer",tc.referer)}
  r.Header.Set("X-Forwarded-Host","weaveos.test");r.Header.Set("X-User-Id","forged")
  if got:=security.ValidSource(r,"https://weaveos.test");got!=tc.want{t.Errorf("origin=%q referer=%q: got %v want %v",tc.origin,tc.referer,got,tc.want)}
 }
}

func TestCSRFRequiresHeaderCookieAndSessionBinding(t *testing.T) {
 token,digest,err:=security.NewCSRFToken();if err!=nil{t.Fatal(err)}
 other,_,err:=security.NewCSRFToken();if err!=nil{t.Fatal(err)}
 for _,tc:=range []struct{cookie,header,hash string;want bool}{
  {token,token,digest,true},{"",token,digest,false},{token,"",digest,false},
  {other,other,digest,false},{token,other,digest,false},{token,token,"",false},
 } {
  r:=httptest.NewRequest("DELETE","https://weaveos.test/api/v1/sessions/current",nil)
  if tc.cookie!=""{r.AddCookie(&http.Cookie{Name:"__Host-csrf",Value:tc.cookie})};r.Header.Set("X-CSRF-Token",tc.header)
  if got:=security.ValidCSRF(r,tc.hash);got!=tc.want{t.Errorf("CSRF binding: got %v want %v",got,tc.want)}
 }
}
