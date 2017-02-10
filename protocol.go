package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

type MsgScanner struct {
	*bufio.Scanner
}

var (
	CRLF   = []byte("\r\n")
	CRLFx2 = []byte("\r\n\r\n")
)

func scanMsg(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, CRLFx2); i >= 0 {
		return i + 4, data[:i+4], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func NewScanner(r io.Reader) *MsgScanner {
	scanner := bufio.NewScanner(r)
	scanner.Split(scanMsg)
	return &MsgScanner{scanner}
}

func (s *MsgScanner) Msg() (*Msg, error) {
	return ParseMsg(s.Bytes())
}

func (s *MsgScanner) Recv() (*Msg, error) {
	if !s.Scan() {
		if s.Err() != nil {
			return nil, s.Err()
		}
		return nil, errors.New("unexpected end of input")
	}
	return s.Msg()
}

type Msg struct {
	Name   string
	Params []Param
}

type Param struct {
	Name  string
	Value string
}

func NewMsg(name string) *Msg {
	return &Msg{name, make([]Param, 0, 8)}
}

func ParseMsg(raw []byte) (*Msg, error) {
	if len(raw) <= 4 {
		return nil, errors.New("len(raw) must greater than 4")
	}
	if !bytes.Equal(raw[len(raw)-4:], CRLFx2) {
		return nil, errors.New("raw must end with '\r\n\r\n'")
	}
	nxt := func(pos, off int) (int, int) {
		return pos + off + 2, bytes.Index(raw[pos+off+2:], CRLF)
	}

	pos, off := 0, bytes.Index(raw, CRLF)

	msg := NewMsg(string(raw[pos : pos+off]))

	for pos, off = nxt(pos, off); off > 0; pos, off = nxt(pos, off) {
		i := bytes.IndexByte(raw[pos:pos+off], ':')
		if i <= 0 {
			return nil, errors.New("invalid param line: " + string(raw[pos:pos+off]))
		}
		msg.Put(string(raw[pos:pos+i]), string(raw[pos+i+1:pos+off]))
	}
	if off != 0 || pos+2 != len(raw) {
		return nil, errors.New("unexpected end of raw")
	}

	return msg, nil
}

func (m *Msg) Get(name string) (string, bool) {
	for _, arg := range m.Params {
		if arg.Name == name {
			return arg.Value, true
		}
	}
	return "", false
}

func (m *Msg) Put(name, value string) {
	m.Params = append(m.Params, Param{name, value})
}

func (m *Msg) Bytes() []byte {
	var buf bytes.Buffer
	buf.WriteString(m.Name)
	buf.Write(CRLF)
	for _, arg := range m.Params {
		buf.WriteString(arg.Name + ":" + arg.Value)
		buf.Write(CRLF)
	}
	buf.Write(CRLF)
	return buf.Bytes()
}
