package server

import (
	"context"
	"distribute/rpc/codec"
	"distribute/rpc/protocol"
	"distribute/rpc/transport"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"sync"
)

type RPCServer interface {
	Register(rcv interface{}, metaData map[string]string) error
	Serve(network string, addr string) error
	Close() error
	Services() []ServiceInfo
}

type ServiceInfo struct {
	Name    string   `json:"name"`
	Methods []string `json:"methods"`
}

/*

	server的设计是，对外提供服务。
	transport接口定义了网络通信的方式，主要包含三个接口， Listen()-监听请求， Accept()-接受请求，建立链接， Close()停止监听
	server聚合了一个transport的具柄， 获得了网络通信的能力
	接受到请求后，使用定一个好的协议将请求解析出来。这里使用的是protocol包下的接口，EncodeMessage(), DecodeMessage()。
	请求解析后，定位到请求的是哪个服务的哪个方法， 采用反射的方式去调用，得到结果。
	得到结果后，将结果填充到响应中，同样采用约定好的协议，将响应封装。
	然后通过网络将响应返回给客户端。
	其中，数据的序列化方式也是需要注意的，这里采用的是codec包提供的接口。
	网络 (transport) -------> 协议(protocol) --------> 执行(反射) --------> 协议(protocol) --------> 网络(transport)
	整个步骤中，从网络到协议，到数据的序列化方式，都是采用接口的形式，方便扩展不同的实现。

*/

type SGServer struct {
	codec      codec.Codec               // 序列化方式
	tr         transport.ServerTransport // 网络传输方式
	serviceMap sync.Map                  // 服务的map
	shutdown   bool
	option     Option // 选项
	mutex      sync.Mutex

	RequestInProcess int64 // 处理中的请求数量
}

func (s *SGServer) Services() []ServiceInfo {

	var services []ServiceInfo
	s.serviceMap.Range(func(key, value interface{}) bool {

		sName := key.(string)
		service, ok := value.(*Service)
		if !ok {
			return false
		}
		var methods []string
		for _, method := range service.methods {
			methods = append(methods, method.method.Name)
		}
		services = append(services, ServiceInfo{sName, methods})
		return true
	})

	return services
}

func NewSGServer(option Option) RPCServer {
	s := &SGServer{}
	s.option = option
	return s
}

type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
}

type Service struct {
	Name    string
	typ     reflect.Type
	rcvr    reflect.Value
	methods map[string]*methodType
}

func (s *SGServer) Register(srv interface{}, metaData map[string]string) error {

	srvType := reflect.TypeOf(srv)
	srvName := srvType.Name()

	service := new(Service)

	service.Name = srvName
	service.typ = srvType
	service.rcvr = reflect.ValueOf(srv)
	service.methods = suitableMethods(srvType, true)
	if len(service.methods) == 0 {
		var errorStr string

		// 如果对应的类型没有任何符合规则的方法，扫描对应的指针类型
		// 也是从net.rpc包里抄来的
		method := suitableMethods(reflect.PtrTo(srvType), false)
		if len(method) != 0 {
			errorStr = "rpcx.Register: type " + srvName + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			errorStr = "rpcx.Register: type " + srvName + " has no exported methods of suitable type"
		}
		fmt.Println(errorStr)
		return errors.New(errorStr)
	}

	_, loaded := s.serviceMap.LoadOrStore(srvName, service)
	if loaded {
		return errors.New("rpc: service already defined:" + srvName)
	}

	return nil
}

// Precompute the reflect type for error. Can't use error directly
// because Typeof takes an empty interface value. This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()
var typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()

//过滤符合规则的方法，从net.rpc包抄的
func suitableMethods(typ reflect.Type, reportErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name

		// 方法必须是可导出的
		if method.PkgPath != "" {
			continue
		}
		// 需要有四个参数: receiver, Context, args, *reply.
		if mtype.NumIn() != 4 {
			if reportErr {
				fmt.Println("method", mname, "has wrong number of ins:", mtype.NumIn())
			}
			continue
		}
		// 第一个参数必须是context.Context
		ctxType := mtype.In(1)
		if !ctxType.Implements(typeOfContext) {
			if reportErr {
				fmt.Println("method", mname, " must use context.Context as the first parameter")
			}
			continue
		}

		// 第二个参数是arg
		argType := mtype.In(2)
		if !isExportedOrBuiltinType(argType) {
			if reportErr {
				fmt.Println(mname, "parameter type not exported:", argType)
			}
			continue
		}
		// 第三个参数是返回值，必须是指针类型的
		replyType := mtype.In(3)
		if replyType.Kind() != reflect.Ptr {
			if reportErr {
				fmt.Println("method", mname, "reply type not a pointer:", replyType)
			}
			continue
		}
		// 返回值的类型必须是可导出的
		if !isExportedOrBuiltinType(replyType) {
			if reportErr {
				fmt.Println("method", mname, "reply type not exported:", replyType)
			}
			continue
		}
		// 必须有一个返回值
		if mtype.NumOut() != 1 {
			if reportErr {
				fmt.Println("method", mname, "has wrong number of outs:", mtype.NumOut())
			}
			continue
		}
		// 返回值类型必须是error
		if returnType := mtype.Out(0); returnType != typeOfError {
			if reportErr {
				fmt.Println("method", mname, "returns", returnType.String(), "not error")
			}
			continue
		}
		methods[mname] = &methodType{method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

func (s *SGServer) WrapperServe(serve ServeFunc) ServeFunc {

	for _, wrapper := range s.option.Wrappers {
		serve = wrapper.WrapperServe(s, serve)
	}

	return serve
}

func (s *SGServer) Serve(network, addr string) error {
	defer func() {
		if err := recover(); err!=nil {
			fmt.Println(err)
		}
	}()
	return (s.WrapperServe(s.serve))(network, addr)
}

func (s *SGServer) WrapperTransport(tp TransportFunc) TransportFunc {

	for _, wrapper := range s.option.Wrappers {
		tp = wrapper.WrapperTransport(s, tp)
	}
	return tp
}

func (s *SGServer) ServeTransport(conn transport.Transport) {
	st := s.serveTransport
	s.WrapperTransport(st)(conn)
}

func (s *SGServer) WrapperDoHandlerRequest(dhr DoHandleRequestFunc) DoHandleRequestFunc {
	for _, wrapper := range s.option.Wrappers {
		dhr = wrapper.WrapperDoHandleRequest(s, dhr)
	}
	return dhr
}

func (s *SGServer) serve(network, addr string) error {

	s.tr = transport.NewServerTransport(s.option.TransportType)
	err := s.tr.Listen(network, addr)
	if err != nil {
		fmt.Println("Listen:", err)
		return err
	}
	for {
		if s.shutdown {
			return nil
		}
		conn, err := s.tr.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") && s.shutdown {
				return nil
			}
			fmt.Println("Accept:", err)
			return err
		}
		go s.ServeTransport(conn)
	}
}

func (s *SGServer) serveTransport(conn transport.Transport) {

	for {
		// 通过Transport拿到消息，并解码得到message信息
		req, err := protocol.NewProtocol(s.option.ProtocolType).DecodeMessage(conn)
		if err != nil {
			fmt.Println(err)
			return
		}
		resp := req.Clone()
		ctx := context.Background()
		s.WrapperDoHandlerRequest(s.doHandleRequest)(ctx, req, resp, conn)
	}
}

func (s *SGServer) doHandleRequest(ctx context.Context, req, resp *protocol.Message, conn transport.Transport) {

	//time.Sleep(2*time.Second)
	// 根据msg获得请求的是哪个服务的哪个接口
	serviceName := req.Header.ServiceName
	methodName := req.Header.MethodName

	// 获得服务信息
	serviceInterface, ok := s.serviceMap.Load(serviceName)
	if !ok {
		s.writeErrorResp(resp, conn, "service not found")
		return
	}
	service, ok := serviceInterface.(*Service)
	if !ok {
		s.writeErrorResp(resp, conn, "not *service type")
		return
	}
	// 获得方法
	method, ok := service.methods[methodName]
	if !ok {
		s.writeErrorResp(resp, conn, "method not found")
		return
	}

	// 参数补充

	arg := newValue(method.ArgType)
	reply := newValue(method.ReplyType)

	// 从请求的data中得到真实的参数
	err := codec.GetCodec(s.option.SerializeType).Decode(req.Data, arg)
	if err != nil {
		s.writeErrorResp(resp, conn, "arg is wrong")
		return
	}
	// 规定方法必然是类型方法，第一个参数是ctx，第二个参数是入参，第三个参数是接受真实的返回值。
	// 如： func (s Service) Add(ctx context.Context, uid int64, info UserInfo) error {}

	// 反射调用方法
	var returns []reflect.Value // returns是方法执行的返回值
	if method.ArgType.Kind() != reflect.Ptr {
		returns = method.method.Func.Call([]reflect.Value{
			service.rcvr,                // 第一个参数必然是服务的类型
			reflect.ValueOf(ctx),        // 第二个参数是ctx
			reflect.ValueOf(arg).Elem(), // 第三个参数是真实参数
			reflect.ValueOf(reply),      // 第四个参数是接受响应数据
		})
	} else {
		returns = method.method.Func.Call([]reflect.Value{
			service.rcvr,
			reflect.ValueOf(ctx),
			reflect.ValueOf(arg),
			reflect.ValueOf(reply),
		})
	}
	// 根据约定，如果返回值不为空，则必定有异常
	if len(returns) != 0 && returns[0].Interface() != nil {
		err := returns[0].Interface().(error)
		s.writeErrorResp(resp, conn, err.Error())
		return
	}

	fmt.Println("reply=",reply)

	data, err := codec.GetCodec(s.option.SerializeType).Encode(reply)
	if err != nil {
		s.writeErrorResp(resp, conn, "codec failed. err="+err.Error())
		return
	}

	resp.Data = data
	resp.StatusCode = protocol.ServiceOKCode

	_, err = conn.Write(protocol.NewProtocol(s.option.ProtocolType).EncodeMessage(resp))
	if err != nil {
		return
	}
}

func (s *SGServer) Close() error {

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.shutdown = true

	// 等所有请求执行完毕，或者到指定时间停止
	ticker := time.NewTicker(time.Duration(s.option.ShutdownWait) * time.Second)
	defer ticker.Stop()
	for {
		if s.RequestInProcess <= 0 {
			break
		}
		select {
		case <-ticker.C:
			break
		}
	}

	return s.tr.Close()
}

func (s *SGServer) writeErrorResp(resp *protocol.Message, tr transport.Transport, errMsg string) {
	resp.Error = errMsg
	resp.StatusCode = protocol.ServiceErrorCode
	tr.Write(protocol.NewProtocol(s.option.ProtocolType).EncodeMessage(resp))
}

func newValue(t reflect.Type) interface{} {
	if t.Kind() == reflect.Ptr {
		return reflect.New(t.Elem()).Interface()
	} else {
		return reflect.New(t).Interface()
	}
}
