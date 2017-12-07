package protocol

import (
	"errors"
	"bytes"
	"encoding/binary"
)

//路由结构
type RouteCT struct {
	//打包结构指针
	packet *Packet
	//路由列表函数指针
	routeFun *RouteFun
}

type RouteFun struct {
	//字符串路由列表
	routes map[string]int16
	//标识对应路由
	rTos map[int16]string
	//int8 对应路由函数
	routesFun map[int16]func(b []byte ,r *RouteCT) error
	//中间件路由函数列表
	useFun []func(b []byte ,r *RouteCT,d string) error
}

//声明新的路由 传入路由列表 生产路由列表
func NewRoute(s []string) (RouteFun , error) {
	//路由不能大于32767个
	if len(s) > 32767 {
		return RouteFun{},errors.New("routes not gt 255")
	}
	var r RouteFun
	//路由长度 开辟内存
	r.routes = make(map[string]int16,len(s))
	//开辟内存
	r.routesFun = make(map[int16]func(b []byte ,r *RouteCT) error)
	for k , v := range s{
		r.routes[v] = int16(k)
	}
	return r,nil
}

//添加中间件
func (routeFun *RouteFun) Use(f func(b []byte ,r *RouteCT,d string) error) error {
	routeFun.useFun = append(routeFun.useFun,f)
	return nil
}

//添加路由 对应回调函数
func (routeFun *RouteFun) Route(r string,f func(b []byte ,r *RouteCT) error) error {
	i , err := routeFun.findRoutes(r)
	if err != nil {
		return errors.New("route not find")
	}
	routeFun.routesFun[i] = f
	return nil
}

//字符路由查找 int8 路由标识
func (routeFun *RouteFun) findRoutes(r string) (int16 , error) {
	i , ok := routeFun.routes[r]
	if ok {
		return i , nil
	}
	return 0 , nil
}

//初始一个新的连接
func (routeFun *RouteFun) NewConn(conn *Packet) RouteCT {
	var ct RouteCT
	ct.packet = conn
	ct.routeFun = routeFun
	return ct
}

//开启路由监听
func (route *RouteCT) Listen() error {
	for  {
		i , b , err := route.Read()
		if err != nil{
			return err
		}
		f , err := route.routeFun.GetFun(i)
		for _ , v := range route.routeFun.useFun {
			err := v(b,route,route.routeFun.rTos[i])
			if err != nil {
				return err
			}
		}
		err = f(b,route)
		if err != nil {
			return err
		}
	}
}

//通过标识获取 路由函数
func (routeFun *RouteFun) GetFun(i int16) (func(b []byte ,r *RouteCT) error, error) {
	f , ok := routeFun.routesFun[i]
	if !ok {
		return nil,errors.New("not find fun")
	}
	return f,nil
}

//读取信息获得路由和详细数据
func (route *RouteCT) Read() (int16 , []byte , error) {
	b , err := route.packet.Read()
	if err != nil {
		 return 0,[]byte(""),err
	}
	buf := bytes.NewBuffer(b)
	var ri int16
	binary.Read(buf,route.packet.Endian,&ri)
	return ri,buf.Bytes(),nil
}

//写入路由和数据
func (route *RouteCT) Write(r string,b []byte) (int , error) {
	i , ok := route.routeFun.routes[r]
	if !ok {
		return 0,errors.New("route not find")
	}
	newBuffer := bytes.NewBuffer([]byte{})
	//写入路由
	binary.Write(newBuffer, route.packet.Endian, i)
	binary.Write(newBuffer, route.packet.Endian, b)
	return route.packet.Write(newBuffer.Bytes())
}