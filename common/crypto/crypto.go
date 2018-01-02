package crypto

import (
	"crypto/aes"
	"errors"
	"io"
	"net"
	"crypto/cipher"
)

type readWriter struct {
	io.Reader
	io.Writer
}

// StreamEncryptDecrypter contains information needed to encrypt/decrypt a
// connection.
type StreamEncryptDecrypter struct {
	EncryptKey []byte
	DecryptKey []byte

	StreamEncrypter func(block cipher.Block, iv []byte) cipher.Stream
	StreamDecrypter func(block cipher.Block, iv []byte) cipher.Stream

	EncryptStream cipher.Stream
	DecryptStream cipher.Stream

	EncryptIV []byte
	DecryptIV []byte
}

func (ed *StreamEncryptDecrypter) initCipherStream() error {
	if ed.EncryptStream == nil {
		if ed.StreamEncrypter == nil || ed.EncryptKey == nil {
			return errors.New("at least one of EncryptStream OR EncryptKey and StreamEncrypter must be set")
		}

		if ed.EncryptIV == nil {
			return errors.New("encrypt IV must be set")
		}

		block, err := aes.NewCipher(ed.EncryptKey)
		if err != nil {
			return err
		}
		ed.EncryptStream = ed.StreamEncrypter(block, ed.EncryptIV)
	}

	if ed.DecryptStream == nil {
		if ed.StreamDecrypter == nil || ed.DecryptKey == nil {
			return errors.New("at least one of DecryptStream OR DecryptKey and StreamDecrypter must be set")
		}

		if ed.DecryptIV == nil {
			return errors.New("decrypt IV must be set")
		}

		block, err := aes.NewCipher(ed.DecryptKey)
		if err != nil {
			return err
		}
		ed.DecryptStream = ed.StreamDecrypter(block, ed.DecryptIV)
	}

	return nil
}

// Ciphertext takes a duplex io.ReadWriter with plaintext, encrypt and return a
// corresponding ciphertext io.ReadWriter. Any ciphertext write to returned
// io.ReadWriter will be decrypted and write to plaintext. Any plaintext read
// from plaintext will be encrypted and write to returned io.ReadWriter.
func (ed *StreamEncryptDecrypter) Ciphertext(plaintext net.Conn) (net.Conn, error) {
	if err := ed.initCipherStream(); err != nil {
		return nil, err
	}

	// logic here could be simpler if golang has a built-in duplex pipe.
	cipherRdIn, cipherWtOut := io.Pipe()
	cipherRdOut, cipherWtIn := io.Pipe()

	ciphertext := &CipherConn{
		&readWriter{
			cipherRdOut,
			cipherWtOut,
		},
		plaintext,
	}

	// decrypt ciphertext to plaintext
	go func() {
		decrypter := &cipher.StreamReader{S: ed.DecryptStream, R: cipherRdIn}
		io.Copy(plaintext, decrypter)
		ciphertext.Close() // which close the underlying plaintext
	}()

	// encrypt plaintext to ciphertext
	go func() {
		encrypter := &cipher.StreamWriter{S: ed.EncryptStream, W: cipherWtIn}
		io.Copy(encrypter, plaintext)
		ciphertext.Close()
	}()

	return ciphertext, nil
}

// Plaintext takes a duplex io.ReadWriter with ciphertext, decrypt and return a
// corresponding plaintext io.ReadWriter. Any plaintext write to returned
// io.ReadWriter will be encrypted and write to ciphertext. Any ciphertext read
// from ciphertext will be decrypted and write to returned io.ReadWriter.
func (ed *StreamEncryptDecrypter) Plaintext(ciphertext net.Conn) (net.Conn, error) {
	if err := ed.initCipherStream(); err != nil {
		return nil, err
	}

	plainRdIn, plainWtOut := io.Pipe()
	plainRdOut, plainWtIn := io.Pipe()

	plaintext := &CipherConn {
		&readWriter{
			plainRdOut,
			plainWtOut,
		},
		ciphertext,
	}

	// encrypt plaintext to ciphertext
	go func() {
		encrypter := &cipher.StreamWriter{S: ed.EncryptStream, W: ciphertext}
		io.Copy(encrypter, plainRdIn)
		plaintext.Close()
	}()

	// decrypt ciphertext to plaintext
	go func() {
		decrypter := &cipher.StreamReader{S: ed.DecryptStream, R: ciphertext}
		io.Copy(plainWtIn, decrypter)
		plaintext.Close()
	}()

	return plaintext, nil
}

// CipherConn implements net.Conn interface, with a underlying io.ReadWriter.
type CipherConn struct {
	io.ReadWriter
	net.Conn
}

func (c *CipherConn) Read(b []byte) (n int, err error) {
	if _, err := c.Conn.Read([]byte{}); err != nil {
		return 0, err
	}
	return c.ReadWriter.Read(b)
}

func (c *CipherConn) Write(b []byte) (n int, err error) {
	if _, err := c.Conn.Write([]byte{}); err != nil {
		return 0, err
	}
	return c.ReadWriter.Write(b)
}
