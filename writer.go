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
func (cw *crlfWriter) Write(p []byte) (n int, err error) {
	crlfBytes := bytes.ReplaceAll(p, []byte{'\n'}, []byte{'\r', '\n'})
	return cw.w.Write(crlfBytes)
}

// countingWriter は、io.Writerをラップし、書き込まれたバイト数をカウントします。
type countingWriter struct {
	w     io.Writer
	count int64
}

// Write は io.Writer インターフェースを実装します。
func (cw *countingWriter) Write(p []byte) (n int, err error) {
	n, err = cw.w.Write(p)
	cw.count += int64(n)
	return n, err
}

// Close は、io.CloserのWriteCloserに対応するために追加します。
// (元のWriterがCloserでなければ何もしない)
func (cw *countingWriter) Close() error {
	if closer, ok := cw.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
