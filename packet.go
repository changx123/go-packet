package packet

import (
	"bytes"
	"encoding/binary"
	"net"
)

type Packet struct {
	//socket连接
	conn net.Conn
	//每次读取Buffer长度 默认1024
	ReadBufferSize int32
	//记录数据缓冲
	ioBuffer *bytes.Buffer
	//记录数据缓冲区现有长度
	iol int32
	//协议长度记录
	l int32
	//读取数据
	b []byte
	//每次写入buffer长度 默认0 如果0不进行分包发送 直接交给socket进行处理
	WriteBufferSize int32
	//端序设置 (binary.BigEndian 大端序 ， binary.LittleEndian 小端序)默认大端序
	Endian binary.ByteOrder
}

//初始化连接
func (packet *Packet) NewConn(conn net.Conn) {
	//设置读取长度默认值
	if packet.ReadBufferSize == 0 {
		packet.ReadBufferSize = 1024
	}
	//设置写入长度默认值
	//if packet.WriteBufferSize == 0 {
	//	packet.WriteBufferSize = 1024
	//}
	//设置端序默认值
	if packet.Endian == nil {
		packet.Endian = binary.BigEndian
	}
	//初始化数据缓冲指针
	if packet.ioBuffer == nil {
		packet.ioBuffer = bytes.NewBuffer([]byte{})
	}
	//分配读取数据变量内存
	packet.b = make([]byte, packet.ReadBufferSize)
	packet.conn = conn
}

//写入数据
func (packet *Packet) Write(b []byte) (int, error) {
	//数据长度
	l := len(b)
	//写入数据长度到开头([] [] [] [] ......)
	buf := lWrite(b, l, packet.Endian)
	//不进行分包发送
	if packet.WriteBufferSize == 0 {
		return packet.conn.Write(buf.Bytes())
	}
	return packet.bWrite(buf, l+4)
}

//写入数据长度
func lWrite(b []byte, l int, endian binary.ByteOrder) *bytes.Buffer {
	newBuffer := bytes.NewBuffer([]byte{})
	//写入长度达文件头
	binary.Write(newBuffer, endian, int32(l))
	//写入数据
	binary.Write(newBuffer, endian, b)
	return newBuffer
}

//发送数据  | 分段数据计算
func (packet *Packet) bWrite(buf *bytes.Buffer, l int) (int, error) {
	//创建取数据变量
	var q []byte
	for {
		//获取缓冲区现有长度
		bl32 := int32(buf.Len())
		//判断缓冲区数据是否够传输长度
		//开辟相应内存进行取值
		if bl32 <= packet.WriteBufferSize {
			q = make([]byte, bl32)
		} else {
			q = make([]byte, packet.WriteBufferSize)
		}
		binary.Read(buf, packet.Endian, &q)
		//发送信息
		_, err := packet.conn.Write(q)
		if err != nil {
			return 0, err
		}
		//如果缓冲区没有数据退出循环
		if buf.Len() <= 0 {
			break
		}
	}
	return l, nil
}

//读取数据
func (packet *Packet) Read() ([]byte, error) {
	for {
		//检查上次是否还存在未获取完的数据
		if packet.l != 0 && packet.iol >= packet.l {
			dL, d := packet.bRead()
			if dL > 0 {
				return d, nil
			}
		}
		l, err := packet.conn.Read(packet.b)
		if err != nil {
			return packet.b, err
		}
		//取到的数据写入缓冲区
		binary.Write(packet.ioBuffer, packet.Endian, packet.b[:l])
		//增加缓冲区长度
		packet.iol += int32(l)
		if packet.iol > 4 {
			dL, d := packet.bRead()
			if dL > 0 {
				return d, nil
			}
		}
	}
}

func (packet *Packet) bRead() (int, []byte) {
	//数据长度为0  获取数据长度
	if packet.l == 0 {
		packet.getPacketLen()
	}
	//缓冲区长度 足够数据长度
	if packet.iol >= packet.l {
		var b []byte
		//开辟需要获取数据的长度
		b = make([]byte, packet.l)
		//取数据
		binary.Read(packet.ioBuffer, packet.Endian, &b)
		l := len(b)
		//如果取到的数据长度和 得到的数据长度吻合返回可用数据
		if int32(l) == packet.l {
			//缓冲区长度减去本次数据长度
			packet.iol -= packet.l
			//本次数据长度重置
			packet.l = 0
			//如果大于4位 取文件头
			if packet.iol >= 4 {
				packet.getPacketLen()
			}
			return l, b
		}
	}
	return 0, []byte{}
}

//获取协议长度
func (packet *Packet) getPacketLen() {
	//获取协议长度
	binary.Read(packet.ioBuffer, packet.Endian, &packet.l)
	packet.iol -= 4
}
