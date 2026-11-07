package evalsample

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/altnovatrail/tutor/internal/newfile"
)

// What a person's sample takes at random (section 11): its ID, and the writer's code that links a
// person's samples with each other. The samples page of canvas makes both; the record mode of the
// board and the converter of the author's sheets make them as it would (R75).
const (
	// codeLength is the length of a writer's code made here, within section 11's 6 to 16.
	codeLength = 12
	// codeAlphabet is what a writer's code is made of: digits and capital Latin letters.
	codeAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// codePattern is a writer's code as section 11 has it.
var codePattern = regexp.MustCompile(`^[0-9A-Z]{6,16}$`)

// lowBase32 writes 16 random bytes as 26 lowercase letters and digits, which an ID of section 11
// takes.
var lowBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewID is a person's sample's ID, at random: 128 bits in 26 lowercase Latin letters and digits.
func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return strings.ToLower(lowBase32.EncodeToString(b[:])), nil
}

// WriterCodeAt is the writer's code kept in the file at path: the author's, which every sample of
// theirs names, the record mode's and the converter's alike. A file that is not there yet is made,
// with a code at random, readable by its owner alone; a file that holds no code of section 11 is an
// error, never written over, since the samples already saved name the code it held.
func WriterCodeAt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		code, err := newCode()
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", err
		}
		switch err := newfile.Write(path, []byte(code+"\n"), 0o600); {
		case errors.Is(err, fs.ErrExist):
			// Another board or converter made it meanwhile: its code is the one.
			return WriterCodeAt(path)
		case err != nil:
			return "", err
		}
		return code, nil
	}
	if err != nil {
		return "", err
	}
	code := strings.TrimSpace(string(data))
	if !codePattern.MatchString(code) {
		return "", fmt.Errorf("evalsample: %s holds no writer's code, 6 to 16 digits and capital Latin letters: mend it by hand, as the samples saved name the code it held", path)
	}
	return code, nil
}

// newCode is a writer's code at random.
func newCode() (string, error) {
	var b strings.Builder
	n := big.NewInt(int64(len(codeAlphabet)))
	for range codeLength {
		k, err := rand.Int(rand.Reader, n)
		if err != nil {
			return "", err
		}
		b.WriteByte(codeAlphabet[k.Int64()])
	}
	return b.String(), nil
}
