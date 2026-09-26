package invitation

type Code struct{Value string;Digest [32]byte}
func Generate()(Code,error){return Code{},nil}
func Digest(string)([32]byte,error){return [32]byte{},nil}
