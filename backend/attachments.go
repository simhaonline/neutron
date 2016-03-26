package backend

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io/ioutil"

	"golang.org/x/crypto/openpgp/packet"
)

// Stores attachments.
type AttachmentsBackend interface {
	// Get an attachment content.
	ReadAttachment(user, id string) (*Attachment, []byte, error)
	// Insert a new attachment.
	InsertAttachment(user string, attachment *Attachment, contents []byte) (*Attachment, error)
	// Delete an attachment.
	DeleteAttachment(user, id string) error
}

// An attachment.
type Attachment struct {
	ID string
	MessageID string `json:",omitempty"`
	Name string
	Size int
	MIMEType string
	KeyPackets string `json:",omitempty"`
	DataPacket string `json:",omitempty"` // TODO: remove this from here
}

type AttachmentKey struct {
	ID string
	Key string
	Algo string
}

// Decrypt a symmetrically encrypted packet with this key.
func (at *AttachmentKey) Decrypt(encrypted []byte) (decrypted []byte, err error) {
	pkt, err := packet.Read(bytes.NewReader(encrypted))
	if err != nil {
		return
	}

	encPkt, ok := pkt.(*packet.SymmetricallyEncrypted)
	if !ok {
		err = errors.New("Packet is not SymmetricallyEncrypted")
		return
	}

	key, err := base64.StdEncoding.DecodeString(at.Key)
	if err != nil {
		return
	}

	// TODO: support more cipher functions
	// See https://godoc.org/golang.org/x/crypto/openpgp/packet#CipherFunction
	var cipherFunc packet.CipherFunction
	switch at.Algo {
	case "aes256":
		cipherFunc = packet.CipherAES256
	default:
		err = errors.New("Unsupported cipher function: "+at.Algo)
		return
	}

	r, err := encPkt.Decrypt(cipherFunc, key)
	if err != nil {
		return
	}
	defer r.Close()

	decrypted, err = ioutil.ReadAll(r)
	return
}
