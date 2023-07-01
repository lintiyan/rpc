package protocol

import (
	"distribute/rpc/codec"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"io"
)

const (
	ErrorProtocolWrong      = "protocol wrong"
	ErrorInvalidTotalLength = "invalid total length"
)

const (
	DefaultProtocolType = iota
)

const (
	ServiceOKCode            = 0
	ServiceErrorCode         = 500
	ServiceShutDownErrorCode = 501
)

const (
	MessageTypeReq MessageType = iota
	MessageTypeResp
)

const (
	RequestSeqKey = "request_seq_key"
)

var protocolMap = map[ProtocolType]Protocol{
	DefaultProtocolType: &RPCProtocol{},
}

func NewProtocol(protocolType ProtocolType) Protocol {
	return protocolMap[protocolType]
}

type (
	ProtocolType byte
	MessageType  byte
	CompressType byte
	StatusCode   int
)

type Header struct {
	Seq           uint64              //序号, 用来唯一标识请求或响应
	MessageType   MessageType         //消息类型，用来标识一个消息是请求还是响应
	CompressType  CompressType        //压缩类型，用来标识一个消息的压缩方式
	SerializeType codec.SerializeType //序列化类型，用来标识消息体采用的编码方式
	StatusCode    StatusCode          //状态类型，用来标识一个请求是正常还是异常
	ServiceName   string              //服务名
	MethodName    string              //方法名
	Error         string              //方法调用发生的异常
	MetaData      map[string]string   //其他元数据
}

type Message struct {
	*Header
	Data []byte
}

type Protocol interface {
	NewMessage() *Message
	EncodeMessage(*Message) []byte
	DecodeMessage(reader io.Reader) (*Message, error)
}

//-------------------------------------------------------------------------------------------------
//|2byte|1byte  |4byte       |4byte        | header length |(total length - header length - 4byte)|
//-------------------------------------------------------------------------------------------------
//|magic|version|total length|header length|     header    |                    body              |
//-------------------------------------------------------------------------------------------------

type RPCProtocol struct {
}

func (r RPCProtocol) NewMessage() *Message {
	var m = &Message{}
	m.Header = &Header{}
	return m
}
func (r RPCProtocol) EncodeMessage(msg *Message) []byte {
	first3bytes := []byte{0xab, 0xba, 0x00}
	headerBytes, err := json.Marshal(msg.Header)
	if err != nil {
		return nil
	}

	totalLen := 4 + len(headerBytes) + len(msg.Data) // header length + header + body
	totalLenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(totalLenBytes, uint32(totalLen))

	var data = make([]byte, totalLen+7) // 7 = magic + version + total length
	var start int
	copyFullWithOffset(data, first3bytes, &start)   //	 添加magic + version
	copyFullWithOffset(data, totalLenBytes, &start) //   添加total length

	headerLenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(headerLenBytes, uint32(len(headerBytes)))

	copyFullWithOffset(data, headerLenBytes, &start) // 添加 header length
	copyFullWithOffset(data, headerBytes, &start)    // 添加 header
	copyFullWithOffset(data, msg.Data, &start)       // 添加data

	return data
}

func copyFullWithOffset(dst []byte, src []byte, start *int) {
	copy(dst[*start:*start+len(src)], src)
	*start = *start + len(src)
}

func (r RPCProtocol) DecodeMessage(reader io.Reader) (*Message, error) {

	// 读取前3字节
	first3Byte := make([]byte, 3)
	_, err := io.ReadFull(reader, first3Byte)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// 校验魔数
	if !checkMagicAndVersion(first3Byte) {
		return nil, errors.New(ErrorProtocolWrong)
	}

	// 获取消息长度
	totalLengthSlice := make([]byte, 4)
	_, err = io.ReadFull(reader, totalLengthSlice)
	if err != nil {
		return nil, err
	}

	// 读取长度
	totalLength := binary.BigEndian.Uint32(totalLengthSlice)
	if totalLength < 4 {
		return nil, errors.New(ErrorInvalidTotalLength)
	}

	data := make([]byte, totalLength)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return nil, err
	}

	// 获取header长度
	headerLenBinary := data[:4]
	headerLen := binary.BigEndian.Uint32(headerLenBinary)
	headerBytes := data[4 : 4+headerLen]
	header := &Header{}

	// 获取header
	err = json.Unmarshal(headerBytes, header)
	if err != nil {
		return nil, err
	}

	// 构建msg
	msg := new(Message)
	msg.Header = header
	msg.Data = data[4+headerLen:]

	return msg, nil
}

func checkMagicAndVersion(b []byte) bool {
	if b[0] == 0xab && b[1] == 0xba {
		return true
	}
	return false
}

func (m *Message) Clone() *Message {
	msg := new(Message)
	header := *m.Header  // 取指针的值，相当于复制一份数据
	msg.Header = &header // 取复制的数据的地址
	return msg
}
