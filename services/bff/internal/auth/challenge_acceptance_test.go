package auth

import (
 "encoding/json"
 "testing"
)

// Accepted ADR-002 requires a Session challenge on every 401 and human-readable messages.
func TestAcceptedUnauthorizedChallengeAndPublicMessages(t *testing.T) {
 a:=setup(t)
 for _, path:=range []string{"/api/v1/sessions","/api/v1/sessions/current"} {
  method:="GET"
  var input any
  if path=="/api/v1/sessions" {method="POST";input=map[string]string{"account":"synthetic-unknown","password":"Aa1!"}}
  response:=a.request(method,path,input,nil,nil)
  if response.Code!=401 {t.Fatalf("want401 got%d",response.Code)}
  if response.Header().Get("WWW-Authenticate")!=`Session realm="enterprise-management-system"` {t.Errorf("%s missing approved401 challenge",path)}
  var body struct {Code,Message string}
  if err:=json.Unmarshal(response.Body.Bytes(),&body);err!=nil {t.Fatal(err)}
  if body.Message=="" || body.Message==body.Code {t.Errorf("%s message must be human-readable, not the machine code",path)}
 }
}
