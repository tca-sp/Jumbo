package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

type NetMsg struct {
	Con net.Conn
	Msg []byte
}

type Network struct {
	IsControlSpeed         bool
	SendSpeed              int //mB/sec
	IsControlLatency       bool
	SendLatency            int
	ControlSpeedMsgChannel chan NetMsg
	amountmax              int
	amountnow              int
	timebeign              time.Time
	isfirsttime            bool
}

func New(IsControlSpeed bool, SendSpeed int, IsControlLatency bool, SendLatency int) Network {
	return Network{
		IsControlSpeed:         IsControlSpeed,
		SendSpeed:              SendSpeed,
		IsControlLatency:       IsControlLatency,
		SendLatency:            SendLatency,
		ControlSpeedMsgChannel: make(chan NetMsg, 1000),
		amountmax:              SendSpeed * 1024 * 1024, //caculate speed by Byte/sec
		amountnow:              0,
		isfirsttime:            true,
	}
}

func (net Network) Init() {

}

func Listen_msg(ip string, backCH chan []byte, silence bool) {
	//IDStr := strconv.Itoa(tp.ID)
	//listenAddress := tp.ListenAddress[:10] + IDStr + tp.ListenAddress[10:]
	var tcpAddr *net.TCPAddr
	tcpAddr, _ = net.ResolveTCPAddr("tcp", ip)
	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		panic(err)
	}
	fmt.Println("Server ready to listen:", ip)
	for {
		tcpConn, err := tcpListener.AcceptTCP()
		if err != nil {
			panic(err)
			continue
		}
		fmt.Println("A replica connected :" + tcpConn.RemoteAddr().String())

		go Handle_con(tcpConn, backCH, silence)

	}

}

func Handle_con(con net.Conn, backCH chan []byte, silence bool) {
	defer con.Close()
	for {
		var size int32
		err1 := binary.Read(con, binary.BigEndian, &size)
		if err1 != nil {
			fmt.Println("error when read size from ",con.RemoteAddr().String(), err1)
			break
		}

		// read from the connection
		var buf = make([]byte, int(size))
		tmpsize := size
		for {
			var tmpbuf = make([]byte, int(tmpsize))
			//log.Println("start to read from conn")
			len, err := con.Read(tmpbuf)
			//fmt.Println("reading------- size:", size, " size now:", n)
			if err != nil {
				fmt.Println("conn read error:", err, con.RemoteAddr().String())
				return
			}
			if len <= 0 {
				fmt.Println("wrong msg : len = 0")
			}

			buf = append(buf[:size-tmpsize], tmpbuf[:len]...)
			if len < int(tmpsize) {
				if !silence {
					fmt.Println("package was cut by ", len)
				}
				tmpsize = tmpsize - int32(len)
			} else {
				break
			}
		}
		if !silence {
			fmt.Println("get a msg from", con.RemoteAddr().String(), "of length:", len(buf))
		}

		msg := buf[:]
		backCH <- msg

	}

}

func Dial(ip string) (net.Conn, error) {
	conn, err := net.Dial("tcp", ip)
	if err != nil {
		return nil, err
	} else {
		return conn, nil
	}
}

func (net Network) Send(con net.Conn, msg []byte) error {
	if net.IsControlSpeed {
		return net.Speed_Control_Send(con, msg)
	} else {
		return net.Direct_Send(con, msg)

	}
}

func (net Network) Speed_Control_Send(con net.Conn, msg []byte) error {

	netmsg := <-net.ControlSpeedMsgChannel
	if net.isfirsttime {
		net.timebeign = time.Now()
		net.isfirsttime = false
	}
	if net.IsControlLatency {
		time.Sleep(time.Millisecond * time.Duration(net.SendLatency))
	}
	err := net.Direct_Send(netmsg.Con, netmsg.Msg)

	elapsed := time.Since(net.timebeign)
	if elapsed >= time.Second {
		net.amountnow = 0
		net.timebeign = time.Now()
	} else {
		net.amountnow += len(netmsg.Msg)
		if net.amountnow >= net.amountmax {
			net.amountnow = 0
			time.Sleep(time.Second - elapsed)
			net.timebeign = time.Now()
		}
	}
	return err

}

/*func (net Network) Speed_Control_Send() {
	amountmax := net.SendSpeed * 1024 * 1024 //caculate speed by Byte/sec
	amountnow := 0
	var timebeign time.Time
	isfirsttime := true
	for {

		netmsg := <-net.ControlSpeedMsgChannel
		if isfirsttime {
			timebeign = time.Now()
			isfirsttime = false
		}
		if net.IsControlLatency {
			time.Sleep(time.Millisecond * time.Duration(net.SendLatency))
		}
		net.Direct_Send(netmsg.Con, netmsg.Msg)
		elapsed := time.Since(timebeign)
		if elapsed >= time.Second {
			amountnow = 0
			timebeign = time.Now()
		} else {
			amountnow += len(netmsg.Msg)
			if amountnow >= amountmax {
				amountnow = 0
				time.Sleep(time.Second - elapsed)
				timebeign = time.Now()
			}
		}

	}
}*/

func (net Network) Direct_Send(con net.Conn, msg []byte) error {
	var buf [4]byte
	m := msg[:]
	binary.BigEndian.PutUint32(buf[:], uint32(len(m)))
	_, err2 := con.Write(buf[:])
	if err2 != nil {
		fmt.Println(err2)
		return err2
	}
	_, err := con.Write(m)
	if err != nil {
		fmt.Println(err2)
		return err2
	}
	return nil
}

func Send2fb(con net.Conn, msg []byte) error {
	var buf [8]byte
	m := msg[:]
	binary.BigEndian.PutUint64(buf[:], uint64(len(m)))
	_, err2 := con.Write(buf[:])
	if err2 != nil {
		panic(err2)
	}
	_, err := con.Write(m)
	if err != nil {
		panic(err)
	}
	return err
}

func Broadcast(cons []net.Conn, msg []byte) error {
	m := msg[:]
	var buf [4]byte
	for _, con := range cons {
		binary.BigEndian.PutUint32(buf[:], uint32(len(m)))
		_, err2 := con.Write(buf[:])
		if err2 != nil {
			panic(err2)
		}
		_, err := con.Write(m)
		if err != nil {
			return err
		}
	}
	return nil
}

func Broadcast_O(id int, cons []net.Conn, msg []byte) error {
	m := msg[:]
	var buf [4]byte
	for i, con := range cons {
		if i != id-1 {
			binary.BigEndian.PutUint32(buf[:], uint32(len(m)))
			_, err2 := con.Write(buf[:])
			if err2 != nil {
				panic(err2)
			}
			_, err := con.Write(m)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func RecvLength(recv net.Conn) (int64, error) {

	var size int64
	err := binary.Read(recv, binary.BigEndian, &size)
	if err != nil {
		fmt.Println("error in readlength")
		panic(err)
	}
	//fmt.Println("size is ", size)
	return size, err
}

func RecvBytes(recv net.Conn) ([]byte, error) {

	size, err := RecvLength(recv)

	if err != nil {
		return nil, err
	}

	buf := make([]byte, size)

	_, err = io.ReadFull(recv, buf)

	if err != nil {
		panic(err)
		return nil, err
	}

	return buf, nil
}

func RecvString(recv net.Conn) (string, error) {

	bytes, err := RecvBytes(recv)

	return string(bytes), err
}
