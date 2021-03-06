// Package kdbx provides basic interfaces to KDBX File Format Library.
//
// KeePass Password Safe is a free and open-source password manager primarily
// for Windows. It officially supports macOS and Linux operating systems
// through the use of Mono. Additionally, there are several unofficial ports
// for Windows Phone, Android, iOS, and BlackBerry devices. KeePass stores
// usernames, passwords, and other fields, including free-form notes and file
// attachments, in an encrypted file. This file can be protected by a master
// password, keyfile, and/or the current Windows account details. By default,
// the KeePass database is stored on a local file system (as opposed to cloud
// storage).
//
// Ref: /usr/share/file/magic/keepass
// Ref: https://en.wikipedia.org/wiki/KeePass
//
//   0000: 03 d9 a2 9a 67 fb 4b b5 01 00 03 00 02 10 00 31  |....g.K........1|
//   0010: c1 f2 e6 bf 71 43 50 be 58 05 21 6a fc 5a ff 03  |....qCP.X.!j.Z..|
//   0020: 04 00 01 00 00 00 04 20 00 e1 0e 5b a9 47 c7 dc  |....... ...[.G..|
//   0030: 51 86 b9 fb f1 4d 6a 6d af 37 09 2d 97 e3 f1 ec  |Q....Mjm.7.-....|
//   0040: a4 88 8b 8e 17 59 65 aa 56 07 10 00 04 38 8b 41  |.....Ye.V....8.A|
//   0050: 2d 0d 96 e9 ed 21 6d 5e 1e 45 68 0c 05 20 00 bc  |-....!m^.Eh.. ..|
//   0060: 42 4c 8d 6c b5 40 1d c8 9e ba 27 68 3f ef ef 55  |BL.l.@....'h?..U|
//   0070: a5 e8 aa 77 4c 83 72 07 25 55 27 f7 f8 79 e8 06  |...wL.r.%U'..y..|
//   0080: 08 00 60 ea 00 00 00 00 00 00 08 20 00 a2 60 65  |..`........ ..`e|
//   0090: 6e bc 67 5b 44 15 4c d8 4d d1 eb 39 6c a0 2f 99  |n.g[D.L.M..9l./.|
//   00a0: 66 79 5c 80 95 fa b6 95 13 5e 7e 1d 23 09 20 00  |fy\......^~.#. .|
//   00b0: 6e 59 a8 c2 12 d6 d9 fa b5 40 9b de 9d 10 4a 2e  |nY.......@....J.|
//   00c0: 74 ce 72 43 95 6d aa 0e 19 25 e4 9b c8 94 e7 bd  |t.rC.m...%......|
//   00d0: 0a 04 00 02 00 00 00 00 04 00 0d 0a 0d 0a        |..............|
package kdbx

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"time"
)

var baseSig = []byte{0x03, 0xd9, 0xa2, 0x9a}
var sndSig1 = []byte{0x65, 0xfb, 0x4b, 0xb5}
var sndSig2 = []byte{0x66, 0xfb, 0x4b, 0xb5}
var sndSig3 = []byte{0x67, 0xfb, 0x4b, 0xb5}

const minor = 1
const major = 3

// Number of bytes for the base signature.
const baseSigLen = 4

// Number of bytes for the secondary signature.
const sndSigXLen = 4

// Number of supported header fields.
const headersLen = 11

// Number of bytes for the master key.
const masterKeyLen = 16

// Determines if the database is compressed.
const compressedFlag = 1

// Number of bytes for the XML blocks: Block ID.
const blockIDLen = 4

// Number of bytes for the XML blocks: Block Hash.
const blockHashLen = 32

// Number of bytes for the XML blocks: Block Data.
const blockDataLen = 4

// Number of bytes for the XML blocks: Block UUID.
const blockUUIDLen = 16

var endHeaderUUID = uint8(0x00) /* endheader id */
var endHeaderData = []byte{0x0d, 0x0a, 0x0d, 0x0a}

// KDBX defines the main library data structure.
type KDBX struct {
	reader     *bufio.Reader
	passphrase []byte
	filename   string
	baseSign   []byte
	scndSign   []byte
	minorVer   uint16
	majorVer   uint16
	headers    []Header
	content    content
}

// Header defines the KDBX file header.
type Header struct {
	id     uint8
	length uint16
	data   []byte
}

// Block defines the XML data portions.
type Block struct {
	id     uint32
	hash   [32]byte
	length uint32
	data   []byte
}

type contentBool bool

type contentUUID [16]byte

type content struct {
	XMLName xml.Name     `xml:"KeePassFile"`
	Meta    *contentMeta `xml:"Meta"`
	Root    *contentRoot `xml:"Root"`
}

type contentMeta struct {
	Generator                  string           `xml:"Generator"`
	HeaderHash                 string           `xml:"HeaderHash"`
	DatabaseName               string           `xml:"DatabaseName"`
	DatabaseNameChanged        *time.Time       `xml:"DatabaseNameChanged"`
	DatabaseDescription        string           `xml:"DatabaseDescription"`
	DatabaseDescriptionChanged *time.Time       `xml:"DatabaseDescriptionChanged"`
	DefaultUserName            string           `xml:"DefaultUserName"`
	DefaultUserNameChanged     *time.Time       `xml:"DefaultUserNameChanged"`
	MaintenanceHistoryDays     string           `xml:"MaintenanceHistoryDays"`
	Color                      string           `xml:"Color"`
	MasterKeyChanged           *time.Time       `xml:"MasterKeyChanged"`
	MasterKeyChangeRec         int64            `xml:"MasterKeyChangeRec"`
	MasterKeyChangeForce       int64            `xml:"MasterKeyChangeForce"`
	MemoryProtection           contentMemProtec `xml:"MemoryProtection"`
	RecycleBinEnabled          contentBool      `xml:"RecycleBinEnabled"`
	RecycleBinUUID             contentUUID      `xml:"RecycleBinUUID"`
	RecycleBinChanged          *time.Time       `xml:"RecycleBinChanged"`
	EntryTemplatesGroup        string           `xml:"EntryTemplatesGroup"`
	EntryTemplatesGroupChanged *time.Time       `xml:"EntryTemplatesGroupChanged"`
	HistoryMaxItems            int64            `xml:"HistoryMaxItems"`
	HistoryMaxSize             int64            `xml:"HistoryMaxSize"`
	LastSelectedGroup          string           `xml:"LastSelectedGroup"`
	LastTopVisibleGroup        string           `xml:"LastTopVisibleGroup"`
	Binaries                   []contentBinary  `xml:"Binaries"`
	CustomData                 string           `xml:"CustomData"`
}

type contentMemProtec struct {
	ProtectNotes    contentBool `xml:"ProtectNotes"`
	ProtectPassword contentBool `xml:"ProtectPassword"`
	ProtectTitle    contentBool `xml:"ProtectTitle"`
	ProtectURL      contentBool `xml:"ProtectURL"`
	ProtectUserName contentBool `xml:"ProtectUserName"`
}

type contentRoot struct {
	Groups         []contentGroup             `xml:"Group"`
	DeletedObjects []contentDeletedObjectData `xml:"DeletedObjects>DeletedObject"`
}

type contentBinary struct {
	ID         int         `xml:"ID,attr"`
	Content    []byte      `xml:",innerxml"`
	Compressed contentBool `xml:"Compressed,attr"`
}

type contentGroup struct {
	UUID                    contentUUID    `xml:"UUID"`
	Name                    string         `xml:"Name"`
	Notes                   string         `xml:"Notes"`
	IconID                  int64          `xml:"IconID"`
	Times                   contentTimes   `xml:"Times"`
	IsExpanded              contentBool    `xml:"IsExpanded"`
	DefaultAutoTypeSequence string         `xml:"DefaultAutoTypeSequence"`
	EnableAutoType          string         `xml:"EnableAutoType"`
	EnableSearching         string         `xml:"EnableSearching"`
	LastTopVisibleEntry     string         `xml:"LastTopVisibleEntry"`
	Entries                 []contentEntry `xml:"Entry,omitempty"`
	Groups                  []contentGroup `xml:"Group,omitempty"`
}

type contentEntry struct {
	UUID            contentUUID      `xml:"UUID"`
	IconID          int64            `xml:"IconID"`
	ForegroundColor string           `xml:"ForegroundColor"`
	BackgroundColor string           `xml:"BackgroundColor"`
	OverrideURL     string           `xml:"OverrideURL"`
	Tags            string           `xml:"Tags"`
	Times           contentTimes     `xml:"Times"`
	Strings         []contentString  `xml:"String,omitempty"`
	AutoType        contentAutoType  `xml:"AutoType"`
	Histories       []contentHistory `xml:"History"`
	Binaries        []contentBinRef  `xml:"Binary,omitempty"`
}

type contentTimes struct {
	LastModificationTime *time.Time  `xml:"LastModificationTime"`
	CreationTime         *time.Time  `xml:"CreationTime"`
	LastAccessTime       *time.Time  `xml:"LastAccessTime"`
	ExpiryTime           *time.Time  `xml:"ExpiryTime"`
	Expires              contentBool `xml:"Expires"`
	UsageCount           int64       `xml:"UsageCount"`
	LocationChanged      *time.Time  `xml:"LocationChanged"`
}

type contentString struct {
	Key   string       `xml:"Key"`
	Value contentValue `xml:"Value"`
}

type contentValue struct {
	Content   string `xml:",chardata"`
	Protected bool   `xml:"Protected,attr,omitempty"`
}

type contentAutoType struct {
	Enabled                 bool                        `xml:"Enabled"`
	DataTransferObfuscation int64                       `xml:"DataTransferObfuscation"`
	Association             *contentAutoTypeAssociation `xml:"Association,omitempty"`
}

type contentAutoTypeAssociation struct {
	Window            string `xml:"Window"`
	KeystrokeSequence string `xml:"KeystrokeSequence"`
}

type contentDeletedObjectData struct {
	XMLName      xml.Name    `xml:"DeletedObject"`
	UUID         contentUUID `xml:"UUID"`
	DeletionTime *time.Time  `xml:"DeletionTime"`
}

type contentHistory struct {
	Entries []contentEntry `xml:"Entry"`
}

type contentBinRef struct {
	Name  string             `xml:"Key"`
	Value contentBinRefValue `xml:"Value"`
}

type contentBinRefValue struct {
	ID int `xml:"Ref,attr"`
}

// New creates and returns a new instance of KDBX.
func New(name string) *KDBX {
	var k KDBX

	k.filename = name
	k.baseSign = make([]byte, baseSigLen)
	k.scndSign = make([]byte, sndSigXLen)
	k.headers = make([]Header, headersLen)

	return &k
}

// EndHeader defines the end limit for the headers block.
func (k *KDBX) EndHeader() []byte {
	return k.headers[0x00].data
}

// Comment is current ignored by KeePass and alternate apps.
func (k *KDBX) Comment() []byte {
	return k.headers[0x01].data
}

// CipherID represents the UUID of the cipher algorithm.
//
// The default cipher is AES-CBC with PKCS7 padding.
func (k *KDBX) CipherID() []byte {
	return k.headers[0x02].data
}

// CompressionFlags determines if the database is compressed or not.
//
// For now, the compression algorithm seems to be GZip, if this header is set
// to 0x01 the payload will need to be decompressed before it can be read.
//
// Not compressed header data:
//
//   []byte{0x00, 0x00, 0x00, 0x00}
func (k *KDBX) CompressionFlags() uint32 {
	return binary.LittleEndian.Uint32(k.headers[0x03].data)
}

// MasterSeed salt to concatenate to the master key.
func (k *KDBX) MasterSeed() []byte {
	return k.headers[0x04].data
}

// TransformSeed seed for AES.Encrypt to generate the master key.
//
// By default, KeePass writes 32 bytes of transform seed.
// Any length is accepted when the key is read from a file.
func (k *KDBX) TransformSeed() []byte {
	return k.headers[0x05].data
}

// TransformRounds number of rounds to compute the master key.
func (k *KDBX) TransformRounds() uint64 {
	return binary.LittleEndian.Uint64(k.headers[0x06].data)
}

// EncryptionIV defines the initialization vector of the cipher.
//
// KeePass always writes 16 bytes of IV, but the length is not checked when
// reading a file. An exception may occur in the encryption engine if the
// database contains the wrong IV length.
//
// An initialization vector (IV) or starting variable (SV) is a fixed-size
// input to a cryptographic primitive that is typically required to be random
// or pseudorandom. Randomization is crucial for encryption schemes to achieve
// semantic security, a property whereby repeated usage of the scheme under
// the same key does not allow an attacker to infer relationships between
// segments of the encrypted message.
func (k *KDBX) EncryptionIV() []byte {
	return k.headers[0x07].data
}

// ProtectedStreamKey used to obfuscate some fields of the decrypted file.
func (k *KDBX) ProtectedStreamKey() []byte {
	return k.headers[0x08].data
}

// StreamStartBytes portion of the decrypted database for verification.
//
// Besides checking if the decryption key is correct, this can also be used to
// check if the file is corrupt before the entire stream is consumed. The data
// should have been randomly generated when the file was saved.
func (k *KDBX) StreamStartBytes() []byte {
	return k.headers[0x09].data
}

// InnerRandomStreamID algorithm used for individual password obfuscation.
//
// Inner stream encryption may be one of these types:
//
// - 0x00: none
// - 0x01: Arc4Variant
// - 0x02: Salsa20
func (k *KDBX) InnerRandomStreamID() uint32 {
	return binary.LittleEndian.Uint32(k.headers[0x0a].data)
}

// IsLockedByNone checks if the passwords are obfuscated by ByNone.
func (k *KDBX) IsLockedByNone() bool {
	return k.InnerRandomStreamID() == 0x00
}

// IsLockedByArc4Variant checks if the passwords are obfuscated by ByArc4Variant.
func (k *KDBX) IsLockedByArc4Variant() bool {
	return k.InnerRandomStreamID() == 0x01
}

// IsLockedBySalsa20 checks if the passwords are obfuscated by BySalsa20.
func (k *KDBX) IsLockedBySalsa20() bool {
	return k.InnerRandomStreamID() == 0x02
}

// FormatVersion returns the version of the file format.
//
// - KeePass file format version 1.x is `0x65`
// - KeePass file format version 2.x is `0x66`
// - KeePass file format version 3.x is `0x67`
func (k *KDBX) FormatVersion() byte {
	return k.scndSign[0]
}

// SetPassphrase defines the database main password.
func (k *KDBX) SetPassphrase(password []byte) {
	hash := sha256.Sum256(password)
	k.passphrase = hash[0:len(hash)]
}

// Decode reads and processes the KDBX file.
func (k *KDBX) Decode() error {
	file, err := os.Open(k.filename)

	if err != nil {
		return err
	}

	defer func() {
		if err := file.Close(); err != nil {
			log.Fatalln("kdbx.decode;", err)
		}
	}()

	k.reader = bufio.NewReader(file)

	if err := k.decodeBaseSignature(); err != nil {
		return err
	}

	if err := k.decodeSecondarySignature(); err != nil {
		return err
	}

	if err := k.decodeMinorVersion(); err != nil {
		return err
	}

	if err := k.decodeMajorVersion(); err != nil {
		return err
	}

	if err := k.decodeFileHeaders(); err != nil {
		return err
	}

	if err := k.decodeFileContent(); err != nil {
		return err
	}

	return k.decodeProtectedEntries()
}

func (k *KDBX) decodeBaseSignature() error {
	if _, err := k.reader.Read(k.baseSign); err != nil {
		return err
	}

	if bytes.Equal(k.baseSign, baseSig) {
		return nil
	}

	return errors.New("invalid base signature")
}

func (k *KDBX) decodeSecondarySignature() error {
	if _, err := k.reader.Read(k.scndSign); err != nil {
		return err
	}

	if bytes.Equal(k.scndSign, sndSig1) {
		/* KeePass file format version 1.x */
		return nil
	}

	if bytes.Equal(k.scndSign, sndSig2) {
		/* KeePass file format version 2.x */
		return nil
	}

	if bytes.Equal(k.scndSign, sndSig3) {
		/* KeePass file format version 3.x */
		return nil
	}

	return errors.New("invalid secondary signature")
}

func (k *KDBX) decodeMinorVersion() error {
	if err := binary.Read(k.reader, binary.LittleEndian, &k.minorVer); err != nil {
		return err
	}

	if k.minorVer == minor {
		return nil
	}

	return errors.New("invalid minor version")
}

func (k *KDBX) decodeMajorVersion() error {
	if err := binary.Read(k.reader, binary.LittleEndian, &k.majorVer); err != nil {
		return err
	}

	if k.majorVer == major {
		return nil
	}

	return errors.New("invalid major version")
}

func (k *KDBX) decodeFileHeaders() error {
	var h Header
	var err error

	for {
		h = Header{} /* reset if there is anything set */

		if err = binary.Read(k.reader, binary.LittleEndian, &h.id); err != nil {
			return errors.New("kdbx.header_id;\x20" + err.Error())
		}

		if err = binary.Read(k.reader, binary.LittleEndian, &h.length); err != nil {
			return errors.New("kdbx.header_length;\x20" + err.Error())
		}

		h.data = make([]byte, h.length)

		if err = binary.Read(k.reader, binary.LittleEndian, &h.data); err != nil {
			return errors.New("kdbx.header_data;\x20" + err.Error())
		}

		if h.id > headersLen {
			return errors.New("kdbx.header_id; unknown header id")
		}

		k.headers[h.id] = h /* header index should be static */

		if h.id == endHeaderUUID && bytes.Equal(h.data, endHeaderData) {
			/* stop reading headers; start reading payload content */
			break
		}
	}

	return nil
}

func (k *KDBX) decodeFileContent() error {
	encrypted, err := ioutil.ReadAll(k.reader)

	if err != nil {
		return err
	}

	mode, err := k.buildDecrypter()

	if err != nil {
		return err
	}

	database := make([]byte, len(encrypted))
	mode.CryptBlocks(database, encrypted)

	expected := k.StreamStartBytes()
	provided := database[0:len(expected)]

	if !bytes.Equal(expected, provided) {
		return errors.New("kdbx.content; invalid auth or corrupt database")
	}

	return k.decodeFileContentXML(database[len(expected):len(database)])
}

func (k *KDBX) decodeFileContentXML(database []byte) error {
	if !k.isCompressed() {
		r := bytes.NewReader(database)
		return xml.NewDecoder(r).Decode(&k.content)
	}

	buf, err := k.decodeFileContentBlocks(database)

	if err != nil {
		return err
	}

	b := bytes.NewBuffer(buf)
	r, err := gzip.NewReader(b)

	if err != nil {
		return err
	}

	defer func() {
		if err := r.Close(); err != nil {
			log.Fatalln("kdbx.xml_blocks;", err)
		}
	}()

	return xml.NewDecoder(r).Decode(&k.content)
}

func (k *KDBX) decodeFileContentBlocks(database []byte) ([]byte, error) {
	var result []byte

	for {
		if len(database) == 0 {
			break
		}

		block, err := k.decodeFileContentBlock(database)

		if err != nil {
			return result, err
		}

		/* xml end block */
		if block.length == 0 {
			return result, nil
		}

		result = append(result, block.data...)

		database = database[k.blockTotalSize()+len(block.data) : len(database)]
	}

	return result, nil
}

func (k *KDBX) decodeFileContentBlock(database []byte) (Block, error) {
	var b Block

	total := k.blockTotalSize()

	if len(database) < total {
		return b, errors.New("kdbx.xml_block; too small")
	}

	var x int
	var y int

	y += blockIDLen
	b.id = binary.LittleEndian.Uint32(database[x:y])

	x += blockIDLen
	y += blockHashLen
	copy(b.hash[0:len(b.hash)], database[x:y])

	x += blockHashLen
	y += blockDataLen
	b.length = binary.LittleEndian.Uint32(database[x:y])

	if b.length == 0 {
		return b, nil
	}

	if len(database)-total < int(b.length) {
		return b, errors.New("kdbx.xml_block; too small")
	}

	b.data = database[total : total+int(b.length)]
	expected := sha256.Sum256(b.data)

	if !bytes.Equal(expected[0:len(expected)], b.hash[0:len(b.hash)]) {
		return b, errors.New("kdbx.xml_block; corrupt")
	}

	return b, nil
}

func (k *KDBX) decodeProtectedEntries() error {
	if k.IsLockedByNone() {
		return nil
	}

	if k.IsLockedByArc4Variant() {
		return nil
	}

	if k.IsLockedBySalsa20() {
		return nil
	}

	return nil
}

func (k *KDBX) isCompressed() bool {
	return k.CompressionFlags() == compressedFlag
}

func (k *KDBX) buildCipher() (cipher.Block, error) {
	key, err := k.buildMasterKey()

	if err != nil {
		return nil, err
	}

	return aes.NewCipher(key)
}

func (k *KDBX) buildDecrypter() (cipher.BlockMode, error) {
	b, err := k.buildCipher()

	if err != nil {
		return nil, err
	}

	return cipher.NewCBCDecrypter(b, k.EncryptionIV()), nil
}

func (k *KDBX) buildCompositeKey() ([]byte, error) {
	hash := sha256.New()

	if _, err := hash.Write(k.passphrase); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func (k *KDBX) buildMasterKey() ([]byte, error) {
	key, err := k.buildCompositeKey()

	if err != nil {
		return nil, err
	}

	b, err := aes.NewCipher(k.TransformSeed())

	if err != nil {
		return nil, err
	}

	var iv []byte

	tr := k.TransformRounds()

	for i := uint64(0); i < tr; i++ {
		iv = make([]byte, masterKeyLen)

		c := cipher.NewCBCEncrypter(b, iv)
		c.CryptBlocks(key[0:masterKeyLen], key[0:masterKeyLen])

		c = cipher.NewCBCEncrypter(b, iv)
		c.CryptBlocks(key[masterKeyLen:len(key)], key[masterKeyLen:len(key)])
	}

	/* [32]byte >>> []byte */
	tmp := sha256.Sum256(key)
	key = tmp[0:len(tmp)]

	key = append(k.MasterSeed(), key...)
	hsh := sha256.Sum256(key)
	key = hsh[0:len(hsh)]

	return key, nil
}

func (k *KDBX) blockTotalSize() int {
	return blockIDLen + blockHashLen + blockDataLen
}

func (u *contentUUID) UnmarshalText(src []byte) error {
	dst := make([]byte, base64.StdEncoding.DecodedLen(len(src)))
	length, err := base64.StdEncoding.Decode(dst, src)

	if err != nil {
		return err
	}

	if length != blockUUIDLen {
		return errors.New("kdbx.xml_block; invalid block uuid")
	}

	copy((*u)[0:len(u)], dst[0:blockUUIDLen])

	return nil
}

func (k *KDBX) Content() content {
	return k.content
}
