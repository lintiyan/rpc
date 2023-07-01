package codec

import "encoding/json"

type SerializeType byte

const (
	JsonSerializeType = iota
)

var m = map[SerializeType]Codec{
	JsonSerializeType: JSONCode{},
}

func GetCodec(st SerializeType) Codec {
	return m[st]
}

type Codec interface {
	Encode(interface{}) ([]byte, error)
	Decode([]byte, interface{}) error
}

type JSONCode struct {
}

func (j JSONCode) Encode(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}

func (j JSONCode) Decode(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
