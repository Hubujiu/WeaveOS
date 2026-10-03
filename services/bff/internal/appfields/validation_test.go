package appfields
import("testing";"encoding/json")
func TestEveryFieldValueAndConfigBoundary(t *testing.T){
 id:="10000000-0000-4000-8000-000000000001";option:="10000000-0000-4000-8000-000000000002";
 cases:=[]struct{kind,config,value,want string}{{"text",`{"maxLength":2}`,`"中文"`,`"中文"`},{"multiline",`{}`,`"a\nb"`,`"a\nb"`},{"boolean",`{}`,`false`,`false`},{"member",`{}`,`"`+id+`"`,`"`+id+`"`},{"department",`{}`,`"`+id+`"`,`"`+id+`"`},{"single_select",`{"options":[{"id":"`+option+`","label":"A"}]}`,`"`+option+`"`,`"`+option+`"`},{"multi_select",`{"options":[{"id":"`+option+`","label":"A"}]}`,`["`+option+`","`+option+`"]`,`["`+option+`"]`}};
 for _,c:=range cases{f:=Field{ID:id,Name:"字段",Kind:c.kind,Default:json.RawMessage("null"),Config:json.RawMessage(c.config)};fs,e:=NormalizeFields([]Field{f});if e!=nil{t.Errorf("valid %s config %v",c.kind,e);continue};got,e:=NormalizeValue(fs[0],json.RawMessage(c.value));if e!=nil||string(got)!=c.want{t.Errorf("%s value want %s got %s %v",c.kind,c.want,got,e)}}
 for _,cfg:=range []string{`null`,`{"scale":null}`,`{"precision":0}`,`{"scale":19}`,`{"scale":2,"roundingPlaces":3}`,`{"precision":3,"scale":4}`,`{"roundingMode":"OTHER"}`,`{"unknown":1}`}{f:=Field{ID:id,Name:"x",Kind:"number",Default:json.RawMessage("null"),Config:json.RawMessage(cfg)};if _,e:=NormalizeFields([]Field{f});e==nil{t.Errorf("invalid config accepted %s",cfg)}}
}
