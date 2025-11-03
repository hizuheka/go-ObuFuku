package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

// outputWriterFactory は、新しい出力先(io.WriteCloser)を生成する関数の型です。
// テスト時にはファイルではなくメモリ上のバッファを返します。
type outputWriterFactory func(part int) (io.WriteCloser, error)

// runSplit はsplitコマンドの公開エントリポイントです。
// 実際のファイルシステム操作を担当します。
func runSplit(splitTag string, maxKB int, inputPath, outputPrefix string) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	// 実際のファイルを作成するファクトリ関数（クロージャ）
	factory := func(part int) (io.WriteCloser, error) {
		filename := fmt.Sprintf("%s%d.xml", outputPrefix, part)
		return os.Create(filename)
	}

	// XML宣言を読み飛ばすリーダー
	bufReader := bufio.NewReader(inputFile)

	// コアロジックの呼び出し
	s, err := newSplitter(bufReader, factory, splitTag, int64(maxKB)*1024)
	if err != nil {
		return err
	}

	return s.process()
}

// splitter はXML分割処理の状態を管理します
type splitter struct {
	decoder *xml.Decoder
	factory outputWriterFactory // os.Createの代わり
	encoder *xml.Encoder

	splitTag string
	maxSize  int64 // バイト単位

	elementStack []xml.StartElement // 親タグの階層
	xmlDecl      []byte             // XML宣言 (<?xml ...?>)

	fileCounter    int
	currentWriter  io.WriteCloser  // *os.File から io.WriteCloser に変更
	currentCounter *countingWriter // サイズ計測用
	currentSize    int64
}

// newSplitter は splitter のコアロジックを初期化します
func newSplitter(reader io.Reader, factory outputWriterFactory, splitTag string, maxSize int64) (*splitter, error) {
	// XML宣言を読み飛ばし、保持します
	bufReader, ok := reader.(*bufio.Reader)
	if !ok {
		bufReader = bufio.NewReader(reader)
	}

	xmlDecl, err := bufReader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read XML declaration: %w", err)
	}

	s := &splitter{
		decoder:      xml.NewDecoder(bufReader),
		factory:      factory,
		splitTag:     splitTag,
		maxSize:      maxSize,
		xmlDecl:      xmlDecl,
		fileCounter:  0,
		elementStack: make([]xml.StartElement, 0),
	}
	return s, nil
}

// process はトークンをループ処理します
func (s *splitter) process() error {
	for {
		token, err := s.decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to decode XML token: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			if err := s.handleStartElement(elem); err != nil {
				return err
			}
		case xml.CharData:
			if err := s.handleCharData(elem); err != nil {
				return err
			}
		case xml.EndElement:
			if err := s.handleEndElement(elem); err != nil {
				return err
			}
		default:
			// コメントや処理命令など
			if s.encoder != nil {
				if err := s.encoder.EncodeToken(elem); err != nil {
					return err
				}
			}
		}
	}

	// 最後のファイルを閉じる
	if s.currentWriter != nil {
		return s.closeCurrentFile()
	}
	return nil
}

// handleStartElement は開始タグを処理します
func (s *splitter) handleStartElement(se xml.StartElement) error {
	// 最初のファイルを開く
	if s.encoder == nil {
		if err := s.openNewFile(); err != nil {
			return err
		}
	}

	s.elementStack = append(s.elementStack, se)
	return s.encoder.EncodeToken(se)
}

// handleCharData はテキストを処理します
func (s *splitter) handleCharData(cd xml.CharData) error {
	if s.encoder == nil {
		return nil // まだ書き込み先がない場合は何もしない
	}

	// 空白のみのテキストノードは破棄する
	// これにより、元のXMLの改行が二重に出力されるのを防ぐ
	if len(strings.TrimSpace(string(cd))) == 0 {
		return nil
	}

	// 意味のあるテキストデータのみを書き込む
	return s.encoder.EncodeToken(cd)
}

// handleEndElement は終了タグを処理します
func (s *splitter) handleEndElement(ee xml.EndElement) error {
	if s.encoder == nil {
		return fmt.Errorf("invalid XML structure: unexpected end element")
	}

	if err := s.encoder.EncodeToken(ee); err != nil {
		return err
	}

	if len(s.elementStack) > 0 {
		s.elementStack = s.elementStack[:len(s.elementStack)-1]
	}

	// このタグが分割タグかチェック
	if ee.Name.Local == s.splitTag {
		if err := s.encoder.Flush(); err != nil {
			return err
		}

		// os.Statの代わりにcountingWriterからサイズを取得
		s.currentSize = s.currentCounter.count

		// サイズ超過、またはmaxSizeが0（＝1タグ1ファイル）の場合、次のファイルへ
		if (s.currentSize >= s.maxSize && s.maxSize > 0) || s.maxSize == 0 {
			// ただし、ルート要素の終了タグの場合は分割しない
			if len(s.elementStack) > 0 {
				if err := s.openNewFile(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// openNewFile は現在のファイルを閉じ、新しいファイルを開きます
func (s *splitter) openNewFile() error {
	// 1. (もしあれば) 古いファイルを閉じる
	if s.currentWriter != nil {
		if err := s.closeCurrentFile(); err != nil {
			return err
		}
	}

	// 2. ファクトリ経由で新しいWriterを作成
	s.fileCounter++
	file, err := s.factory(s.fileCounter)
	if err != nil {
		return err
	}
	s.currentWriter = file // io.WriteCloserを保持

	// 3. ライターとエンコーダーを設定
	s.currentCounter = &countingWriter{w: file} // countingWriterでラップ
	writer := newCRLFWriter(s.currentCounter)   // crlfWriterでラップ
	s.encoder = xml.NewEncoder(writer)
	s.encoder.Indent("", "  ")

	// 4. XML宣言を書き込む
	if _, err := writer.Write(s.xmlDecl); err != nil {
		return err
	}

	// 5. 親タグの階層（ヘッダー）を書き込む
	for _, startElem := range s.elementStack {
		if err := s.encoder.EncodeToken(startElem); err != nil {
			return err
		}
	}

	s.currentSize = 0 // リセット
	return nil
}

// closeCurrentFile は親タグの終了タグを書き込み、ファイルを閉じます
func (s *splitter) closeCurrentFile() error {
	// 親タグのスタックを逆順にたどり、終了タグを書き込む
	for i := len(s.elementStack) - 1; i >= 0; i-- {
		elem := s.elementStack[i]
		if err := s.encoder.EncodeToken(xml.EndElement{Name: elem.Name}); err != nil {
			return err
		}
	}

	if err := s.encoder.Flush(); err != nil {
		return err
	}

	return s.currentWriter.Close()
}
