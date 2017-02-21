package main

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/howeyc/gopass"
	"github.com/twinj/uuid"
	"github.com/zyguan/just"
)

const (
	SERVER_ADDR = "172.22.1.144:10001"
	RSA_KEY     = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArM43Q1ctTQ8pHp5dW8xk
Fm5hieEzm92MBx6M1uVf8Va3Qrt5rLcXK+YbFUyN/oAFB5hopx0QbWOM2hiohvxp
I+HB6rh5p/Q/Ywmm1tA3T/GdvttzFjhAyDnnTiY/O61m+hoEivavDcxLtkZ4dNy/
n1feI7zDc61LP40S+AG5+Qby6HyNetkWC8h01FwW8Hm3CY6vfEDJ3HPsqDKMnUaX
/PqoKv8f2sUFl/mcQz18LH0JNwND4qNUqI+BqpNKJsutpkOB6dGA9dXQtTGc2bzo
5IPxGsSrxJS01TSjqPoASoRj8YKVISJHHwkVbun+r5wx5OLtEFcMxxh3LELgIWDk
aQIDAQAB
-----END PUBLIC KEY-----`
)

var pk *rsa.PublicKey

func init() {
	block, _ := pem.Decode([]byte(RSA_KEY))
	key, _ := x509.ParsePKIXPublicKey(block.Bytes)
	pk = key.(*rsa.PublicKey)
}

func encrypt(data []byte) string {
	out, err := rsa.EncryptPKCS1v15(rand.Reader, pk, data)
	if err != nil {
		log.Fatal("encrypt: " + err.Error())
	}
	return hex.EncodeToString(out)
}

func pushTime(sid string, conn net.Conn) string {
	addr := conn.LocalAddr().String()
	ip := addr[:strings.LastIndex(addr, ":")]
	sum := md5.Sum([]byte(fmt.Sprintf("liuyan:%s:%s", sid, ip)))
	return hex.EncodeToString(sum[:])
}

func try(tag string) func(interface{}, error) interface{} {
	return just.TryTo(tag)
}

func main() {
	defer just.CatchF(func(err error) error {
		log.Fatal(err)
		return nil
	})(nil)

	var user string
	var pass []byte

	switch len(os.Args) {
	case 2:
		user = os.Args[1]
		fmt.Print("Password: ")
		pass = try("get password")(gopass.GetPasswd()).([]byte)
	case 3:
		user = os.Args[1]
		pass = []byte(os.Args[2])
	default:
		fmt.Printf("Usage: %s username [password]\n", os.Args[0])
		os.Exit(1)
	}

	log.Print("try to obtain authorization from bnac server")
	conn := try("dial to server")(net.Dial("tcp", SERVER_ADDR)).(net.Conn)

	in := NewScanner(conn)

	// ASK_ENCODE
	req := NewMsg("ASK_ENCODE")
	req.Put("PLATFORM", "MAC")
	req.Put("VERSION", "1.0.1.22")
	req.Put("CLIENTID", "{"+strings.ToUpper(uuid.NewV4().String())+"}")
	try("[ASK_ENCODE] send")(conn.Write(req.Bytes()))

	res := try("[ASK_ENCODE] recv")(in.Recv()).(*Msg)
	if res.Name != "601" {
		log.Fatal("[ASK_ENCODE] bad response code: " + res.Name)
	}

	// OPEN_SESAME
	req = NewMsg("OPEN_SESAME")
	req.Put("SESAME_MD5", "INVALID MD5")
	try("[OPEN_SESAME] send")(conn.Write(req.Bytes()))

	res = try("[OPEN_SESAME] recv")(in.Recv()).(*Msg)
	if res.Name != "603" {
		log.Fatal("[OPEN_SESAME] bad response code: " + res.Name)
	}

	// SESAME_VALUE
	req = NewMsg("SESAME_VALUE")
	req.Put("VALUE", "0")
	try("[SESAME_VALUE] send")(conn.Write(req.Bytes()))

	res = try("[SESAME_VALUE] recv")(in.Recv()).(*Msg)
	if res.Name != "604" {
		log.Fatal("[SESAME_VALUE] bad response code: " + res.Name)
	}

	// AUTH
	req = NewMsg("AUTH")
	req.Put("OS", "MAC")
	req.Put("USER", user)
	req.Put("PASS", encrypt(pass))
	req.Put("AUTH_TYPE", "DOMAIN")
	try("[AUTH] send")(conn.Write(req.Bytes()))

	res = try("[AUTH] recv")(in.Recv()).(*Msg)
	if res.Name != "288" {
		log.Fatal("[AUTH] bad response code: " + res.Name)
	}

	sid, ok := res.Get("SESSION_ID")
	if !ok {
		log.Fatal("[AUTH] SESSION_ID is not found in response")
	}
	role, ok := res.Get("ROLE")
	if !ok {
		log.Fatal("[AUTH] ROLE is not found in response")
	}

	// PUSH
	req = NewMsg("PUSH")
	req.Put("TIME", pushTime(sid, conn))
	req.Put("SESSIONID", sid)
	req.Put("ROLE", role)
	try("[AUTH] send")(conn.Write(req.Bytes()))

	res = try("[AUTH] recv")(in.Recv()).(*Msg)
	if res.Name != "220" {
		log.Fatal("[AUTH] bad response code: " + res.Name)
	}

	conn.Close()
	log.Printf("welcome to baidu, %s!", user)

	// heartbeats
	hbCnt := 1
	for {
		time.Sleep(time.Minute)
		go func(idx int) {
			log.Printf("send heartbeat #%d", idx)
			conn, err := net.Dial("udp", SERVER_ADDR)
			if err != nil {
				log.Print("failed to connect to server for heartbeat: ", err)
				return
			}
			defer conn.Close()

			req := NewMsg("KEEP_ALIVE")
			req.Put("SESSIONID", sid)
			req.Put("USER", user)
			req.Put("AUTH_TYPE", "DOMAIN")
			req.Put("HEARTBEAT_INDEX", strconv.Itoa(idx))
			_, err = conn.Write(req.Bytes())
			if err != nil {
				log.Print("failed to send heartbeat: ", err)
			}
		}(hbCnt)
		hbCnt += 1
	}
}
