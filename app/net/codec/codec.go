package codec

import (
	"encoding/binary"
	"github.com/orbit-w/golib/bases/packet"
	"github.com/orbit-w/ogateway/app/net/onet"
	"io"
	"log"
	"net"
	"time"
)

/*
   @Author: orbit-w
   @File: codec
   @2023 12月 周六 20:41
*/

const (
	gzipSize = 1
)

// Codec 通用流式数据编解码器
type Codec struct {
	isGzip          bool //压缩标识符（建议超过100byte消息进行压缩）
	maxIncomingSize uint32
}

func NewCodec(max uint32, _isGzip bool) *Codec {
	return &Codec{
		isGzip:          _isGzip,
		maxIncomingSize: max,
	}
}

// EncodeBody 消息编码协议 body: size<int32> | gzipped<bool> | body<bytes>
func (c *Codec) EncodeBody(body packet.IPacket, gzipped bool) (packet.IPacket, error) {
	defer body.Return()
	buf := packet.Writer()
	writer := func(data []byte) {
		buf.WriteInt32(int32(len(data)) + gzipSize)
		buf.WriteBool(gzipped)
		buf.Write(data)
	}

	if gzipped {
		compressed, err := EncodeGzip(body.Data())
		if err != nil {
			log.Println("[Codec] [func:encodeBody] encode gzip failed: ", err.Error())
			return nil, err
		}
		writer(compressed)
	} else {
		writer(body.Data())
	}

	return buf, nil
}

func (c *Codec) BlockDecodeBody(conn net.Conn, header, body []byte) (packet.IPacket, error) {
	err := conn.SetReadDeadline(time.Now().Add(ReadTimeout))
	if err != nil {
		return nil, err
	}

	_, err = io.ReadFull(conn, header)
	if err != nil {
		if err != io.EOF && !onet.IsClosedConnError(err) {
			log.Println("[Codec] [func:BlockDecodeBody] receive data head failed: ", err.Error())
		}
		return nil, err
	}

	size := binary.BigEndian.Uint32(header)
	if size > c.maxIncomingSize {
		return nil, onet.ExceedMaxIncomingPacket(size)
	}

	body = body[:size]
	if _, err = io.ReadFull(conn, body); err != nil {
		return nil, onet.ReadBodyFailed(err)
	}

	buf := packet.Reader(body)
	return c.decodeBody(buf)
}

func (c *Codec) decodeBody(buf packet.IPacket) (packet.IPacket, error) {
	gzipped, err := buf.ReadBool()
	if err != nil {
		return nil, err
	}

	if !gzipped {
		return buf, nil
	}
	return DecodeGzip(buf)
}

// body: size<int32> | gzipped<byte> | body<bytes>
func (c *Codec) encodeBody(buf, body packet.IPacket, gzipped bool) {
	size := body.Len()
	buf.WriteInt32(int32(size) + gzipSize)
	buf.WriteBool(gzipped)
	if gzipped {
		compressed, err := EncodeGzip(body.Data())
		if err != nil {
			log.Println("[Codec] [func:encodeBody] encode gzip failed: ", err.Error())
			return
		}
		buf.Write(compressed)
		return
	}
	buf.Write(body.Data())
}

func (c *Codec) checkPacketSize(header []byte) error {
	if size := binary.BigEndian.Uint32(header); size > c.maxIncomingSize {
		return onet.ExceedMaxIncomingPacket(size)
	}
	return nil
}
