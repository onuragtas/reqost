package script

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"hash"

	"github.com/dop251/goja"
	"golang.org/x/crypto/pbkdf2"
)

// registerCrypto wires the Go-implemented digest/cipher primitives that back
// the CryptoJS shim (prelude.go) onto __host. Every hook here takes/returns
// hex-encoded bytes rather than plain JS strings: the JS side represents both
// raw UTF-8 messages and WordArray results as hex so the same functions can
// be composed (e.g. HMAC of a hash's output) without ambiguity about what a
// string argument actually contains — exactly what real CryptoJS's WordArray
// type does internally.
func registerCrypto(host *goja.Object) {
	digest := func(newHash func() hash.Hash) func(string) string {
		return func(hexMsg string) string {
			b, _ := hex.DecodeString(hexMsg)
			h := newHash()
			h.Write(b)
			return hex.EncodeToString(h.Sum(nil))
		}
	}
	hmacDigest := func(newHash func() hash.Hash) func(string, string) string {
		return func(hexMsg, hexKey string) string {
			msg, _ := hex.DecodeString(hexMsg)
			key, _ := hex.DecodeString(hexKey)
			m := hmac.New(newHash, key)
			m.Write(msg)
			return hex.EncodeToString(m.Sum(nil))
		}
	}

	_ = host.Set("md5Hex", digest(md5.New))
	_ = host.Set("sha1Hex", digest(sha1.New))
	_ = host.Set("sha224Hex", digest(sha256.New224))
	_ = host.Set("sha256Hex", digest(sha256.New))
	_ = host.Set("sha384Hex", digest(sha512.New384))
	_ = host.Set("sha512Hex", digest(sha512.New))

	_ = host.Set("hmacMD5Hex", hmacDigest(md5.New))
	_ = host.Set("hmacSHA1Hex", hmacDigest(sha1.New))
	_ = host.Set("hmacSHA224Hex", hmacDigest(sha256.New224))
	_ = host.Set("hmacSHA256Hex", hmacDigest(sha256.New))
	_ = host.Set("hmacSHA384Hex", hmacDigest(sha512.New384))
	_ = host.Set("hmacSHA512Hex", hmacDigest(sha512.New))

	// PBKDF2: keyLenBytes/iterations mirror CryptoJS.PBKDF2's keySize (in
	// 32-bit words, so the JS shim multiplies by 4) / iterations config.
	// CryptoJS defaults to a SHA1 hasher when none is given.
	_ = host.Set("pbkdf2Hex", func(hexPass, hexSalt string, iterations, keyLenBytes int, hasherName string) string {
		pass, _ := hex.DecodeString(hexPass)
		salt, _ := hex.DecodeString(hexSalt)
		if iterations <= 0 {
			iterations = 1
		}
		if keyLenBytes <= 0 {
			keyLenBytes = 16
		}
		key := pbkdf2.Key(pass, salt, iterations, keyLenBytes, hasherByName(hasherName))
		return hex.EncodeToString(key)
	})

	_ = host.Set("randomHex", func(n int) string {
		if n <= 0 {
			return ""
		}
		b := make([]byte, n)
		_, _ = crand.Read(b)
		return hex.EncodeToString(b)
	})

	_ = host.Set("utf8ToHex", func(s string) string { return hex.EncodeToString([]byte(s)) })
	_ = host.Set("hexToUtf8", func(hexStr string) string {
		b, err := hex.DecodeString(hexStr)
		if err != nil {
			return ""
		}
		return string(b)
	})
	_ = host.Set("hexToBase64", func(hexStr string) string {
		b, err := hex.DecodeString(hexStr)
		if err != nil {
			return ""
		}
		return base64.StdEncoding.EncodeToString(b)
	})
	_ = host.Set("base64ToHex", func(b64 string) string {
		b, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return ""
		}
		return hex.EncodeToString(b)
	})

	// evpBytesToKeyHex replicates OpenSSL's legacy MD5-based KDF, which is
	// what CryptoJS's passphrase-mode AES/DES/TripleDES use under the hood
	// (CryptoJS.EvpKDF) to derive a key+iv from a password and an 8-byte
	// salt — this is what makes `CryptoJS.AES.encrypt(msg, "secret")` output
	// interoperable with `openssl enc -aes-256-cbc -k secret`.
	_ = host.Set("evpBytesToKeyHex", func(hexPass, hexSalt string, keyLenBytes, ivLenBytes int) string {
		pass, _ := hex.DecodeString(hexPass)
		salt, _ := hex.DecodeString(hexSalt)
		key, iv := evpBytesToKey(pass, salt, keyLenBytes, ivLenBytes)
		return hex.EncodeToString(key) + ":" + hex.EncodeToString(iv)
	})

	_ = host.Set("aesEncryptHex", func(hexMsg, hexKey, hexIV, mode, padding string) map[string]any {
		return blockEncrypt(newAESCipher, hexMsg, hexKey, hexIV, mode, padding)
	})
	_ = host.Set("aesDecryptHex", func(hexCt, hexKey, hexIV, mode, padding string) map[string]any {
		return blockDecrypt(newAESCipher, hexCt, hexKey, hexIV, mode, padding)
	})
	_ = host.Set("desEncryptHex", func(hexMsg, hexKey, hexIV, mode, padding string) map[string]any {
		return blockEncrypt(newDESCipher, hexMsg, hexKey, hexIV, mode, padding)
	})
	_ = host.Set("desDecryptHex", func(hexCt, hexKey, hexIV, mode, padding string) map[string]any {
		return blockDecrypt(newDESCipher, hexCt, hexKey, hexIV, mode, padding)
	})
	_ = host.Set("tripleDesEncryptHex", func(hexMsg, hexKey, hexIV, mode, padding string) map[string]any {
		return blockEncrypt(newTripleDESCipher, hexMsg, hexKey, hexIV, mode, padding)
	})
	_ = host.Set("tripleDesDecryptHex", func(hexCt, hexKey, hexIV, mode, padding string) map[string]any {
		return blockDecrypt(newTripleDESCipher, hexCt, hexKey, hexIV, mode, padding)
	})
}

func hasherByName(name string) func() hash.Hash {
	switch name {
	case "MD5":
		return md5.New
	case "SHA1":
		return sha1.New
	case "SHA224":
		return sha256.New224
	case "SHA384":
		return sha512.New384
	case "SHA512":
		return sha512.New
	default:
		return sha256.New
	}
}

// evpBytesToKey is OpenSSL's EVP_BytesToKey with digest=MD5, count=1 (the
// fixed parameters CryptoJS's OpenSSL-compatible KDF uses).
func evpBytesToKey(password, salt []byte, keyLen, ivLen int) (key, iv []byte) {
	var out, prev []byte
	for len(out) < keyLen+ivLen {
		h := md5.New()
		h.Write(prev)
		h.Write(password)
		h.Write(salt)
		prev = h.Sum(nil)
		out = append(out, prev...)
	}
	return out[:keyLen], out[keyLen : keyLen+ivLen]
}

type newBlockCipherFunc func(key []byte) (cipher.Block, error)

func newAESCipher(key []byte) (cipher.Block, error) { return aes.NewCipher(key) }
func newDESCipher(key []byte) (cipher.Block, error) { return des.NewCipher(key) }

// newTripleDESCipher accepts a 16-byte key as EDE2 (reusing the first 8 bytes
// as the third key) in addition to the standard 24-byte EDE3 key, matching
// CryptoJS/OpenSSL's convention for 2-key triple DES.
func newTripleDESCipher(key []byte) (cipher.Block, error) {
	if len(key) == 16 {
		key = append(append([]byte{}, key...), key[:8]...)
	}
	return des.NewTripleDESCipher(key)
}

func pkcs7Pad(b []byte, blockSize int) []byte {
	padLen := blockSize - len(b)%blockSize
	out := make([]byte, len(b)+padLen)
	copy(out, b)
	for i := len(b); i < len(out); i++ {
		out[i] = byte(padLen)
	}
	return out
}

func pkcs7Unpad(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 || len(b)%blockSize != 0 {
		return nil, errors.New("invalid padded data")
	}
	padLen := int(b[len(b)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(b) {
		return nil, errors.New("invalid padding")
	}
	for _, c := range b[len(b)-padLen:] {
		if int(c) != padLen {
			return nil, errors.New("invalid padding")
		}
	}
	return b[:len(b)-padLen], nil
}

func errResult(msg string) map[string]any { return map[string]any{"ok": false, "error": msg} }

func blockEncrypt(newCipher newBlockCipherFunc, hexMsg, hexKey, hexIV, mode, padding string) map[string]any {
	msg, err := hex.DecodeString(hexMsg)
	if err != nil {
		return errResult("invalid message")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return errResult("invalid key")
	}
	block, err := newCipher(key)
	if err != nil {
		return errResult(err.Error())
	}
	bs := block.BlockSize()
	if padding != "NoPadding" {
		msg = pkcs7Pad(msg, bs)
	} else if len(msg)%bs != 0 {
		return errResult("message length is not a multiple of the block size (NoPadding)")
	}
	ct := make([]byte, len(msg))
	if mode == "ECB" {
		for i := 0; i < len(msg); i += bs {
			block.Encrypt(ct[i:i+bs], msg[i:i+bs])
		}
	} else {
		iv, err := hex.DecodeString(hexIV)
		if err != nil || len(iv) != bs {
			return errResult("invalid iv")
		}
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, msg)
	}
	return map[string]any{"ok": true, "value": hex.EncodeToString(ct)}
}

func blockDecrypt(newCipher newBlockCipherFunc, hexCt, hexKey, hexIV, mode, padding string) map[string]any {
	ct, err := hex.DecodeString(hexCt)
	if err != nil {
		return errResult("invalid ciphertext")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return errResult("invalid key")
	}
	block, err := newCipher(key)
	if err != nil {
		return errResult(err.Error())
	}
	bs := block.BlockSize()
	if len(ct) == 0 || len(ct)%bs != 0 {
		return errResult("ciphertext length is not a multiple of the block size")
	}
	pt := make([]byte, len(ct))
	if mode == "ECB" {
		for i := 0; i < len(ct); i += bs {
			block.Decrypt(pt[i:i+bs], ct[i:i+bs])
		}
	} else {
		iv, err := hex.DecodeString(hexIV)
		if err != nil || len(iv) != bs {
			return errResult("invalid iv")
		}
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, ct)
	}
	if padding != "NoPadding" {
		pt, err = pkcs7Unpad(pt, bs)
		if err != nil {
			return errResult(err.Error())
		}
	}
	return map[string]any{"ok": true, "value": hex.EncodeToString(pt)}
}
