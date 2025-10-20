package main

import (
	"bytes"
	"io"
)

// crlfWriter は、io.Writerをラップし、LF ('\n') の改行コードを
// CRLF ('\r\n') に置換します。
type crlfWriter struct {
	w io.Writer
}

// newCRLFWriter は、CRLF改行コードを保証する新しいWriterを作成します。
func newCRLFWriter(w io.Writer) *crlfWriter {
	return &crlfWriter{w: w}
}

// Write は io.Writer インターフェースを実装します。
// 書き込まれるデータ内のLFをCRLFに置換してから、元のWriterに渡します。
func (cw *crlfWriter) Write(p []byte) (n int, err error) {
	// \n を \r\n に置換
	crlfBytes := bytes.ReplaceAll(p, []byte{'\n'}, []byte{'\r', '\n'})
	return cw.w.Write(crlfBytes)
}
