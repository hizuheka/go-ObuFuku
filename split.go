package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// splitter はXML分割処理の状態を管理します
type splitter struct {
	decoder *xml.Decoder
	writer  io.Writer // newCRLFWriterでラップされたもの
	encoder *xml.Encoder

	splitTag  string
	maxSize   int64 // バイト単位
	outPrefix string

	elementStack []xml.StartElement // 親タグの階層
	xmlDecl      []byte             // XML宣言 (<?xml ...?>)

	fileCounter int
	currentFile *os.File
	currentSize int64
}

// runSplit はsplitコマンドのエントリポイントです
func runSplit(splitTag string, maxKB int, inputPath, outputPrefix string) error {
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	// XML宣言を読み飛ばし、保持します
	bufReader := bufio.NewReader(inputFile)
	xmlDecl, err := bufReader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read XML declaration: %w", err)
	}

	s := &splitter{
		decoder:      xml.NewDecoder(bufReader), // 宣言を読み飛ばしたリーダーを使用
		splitTag:     splitTag,
		maxSize:      int64(maxKB) * 1024,
		outPrefix:    outputPrefix,
		xmlDecl:      xmlDecl,
		fileCounter:  0,
		elementStack: make([]xml.StartElement, 0),
	}

	return s.process()
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
	if s.currentFile != nil {
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

	// スタックに積む
	s.elementStack = append(s.elementStack, se)

	// エンコーダーに書き込む
	return s.encoder.EncodeToken(se)
}

// handleCharData はテキストを処理します
func (s *splitter) handleCharData(cd xml.CharData) error {
	if s.encoder != nil {
		return s.encoder.EncodeToken(cd)
	}
	return nil
}

// handleEndElement は終了タグを処理します
func (s *splitter) handleEndElement(ee xml.EndElement) error {
	if s.encoder == nil {
		return fmt.Errorf("invalid XML structure: unexpected end element")
	}

	// エンコーダーに書き込む
	if err := s.encoder.EncodeToken(ee); err != nil {
		return err
	}

	// スタックからポップ
	if len(s.elementStack) > 0 {
		s.elementStack = s.elementStack[:len(s.elementStack)-1]
	}

	// このタグが分割タグかチェック
	if ee.Name.Local == s.splitTag {
		// 分割タグが完了した時点でファイルサイズをチェック
		if err := s.encoder.Flush(); err != nil {
			return err
		}

		stat, err := s.currentFile.Stat()
		if err != nil {
			return err
		}
		s.currentSize = stat.Size()

		// サイズ超過、またはmaxSizeが0（＝1タグ1ファイル）の場合、次のファイルへ
		if s.currentSize >= s.maxSize && s.maxSize > 0 {
			if err := s.openNewFile(); err != nil {
				return err
			}
		}
	}
	return nil
}

// openNewFile は現在のファイルを閉じ、新しいファイルを開きます
func (s *splitter) openNewFile() error {
	// 1. (もしあれば) 古いファイルを閉じる
	if s.currentFile != nil {
		if err := s.closeCurrentFile(); err != nil {
			return err
		}
	}

	// 2. 新しいファイルを作成
	s.fileCounter++
	filename := fmt.Sprintf("%s%d.xml", s.outPrefix, s.fileCounter)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	s.currentFile = file

	// 3. ライターとエンコーダーを設定
	s.writer = newCRLFWriter(file) // 既存のcrlfWriter.goを活用
	s.encoder = xml.NewEncoder(s.writer)
	s.encoder.Indent("", "  ") // 整形

	// 4. XML宣言を書き込む
	if _, err := s.writer.Write(s.xmlDecl); err != nil {
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

	return s.currentFile.Close()
}
