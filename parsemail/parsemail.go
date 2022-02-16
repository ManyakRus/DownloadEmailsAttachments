package parsemail

import (
	"bytes"
	"encoding/base64"
	b64 "encoding/base64" //sanek
	"fmt"
	"golang.org/x/text/encoding/charmap" //sanek
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

const contentTypeMultipartMixed = "multipart/mixed"
const contentTypeMultipartAlternative = "multipart/alternative"
const contentTypeMultipartRelated = "multipart/related"
const contentTypeTextHtml = "text/html"
const contentTypeTextPlain = "text/plain"

// Parse an email message read from io.Reader into parsemail.Email struct
func Parse(r io.Reader) (email Email, err error) {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return
	}

	email, err = createEmailFromHeader(msg.Header)
	if err != nil {
		return
	}

	email.ContentType = msg.Header.Get("Content-Type")
	contentType, params, err := parseContentType(email.ContentType)
	if err != nil {
		return
	}

	switch contentType {
	case contentTypeMultipartMixed:
		email.TextBody, email.HTMLBody, email.Attachments, email.EmbeddedFiles, err = parseMultipartMixed(msg.Body, params["boundary"])
	case contentTypeMultipartAlternative:
		email.TextBody, email.HTMLBody, email.EmbeddedFiles, err = parseMultipartAlternative(msg.Body, params["boundary"])
	case contentTypeMultipartRelated:
		email.TextBody, email.HTMLBody, email.EmbeddedFiles, err = parseMultipartRelated(msg.Body, params["boundary"])
	case contentTypeTextPlain:
		message, _ := ioutil.ReadAll(msg.Body)
		email.TextBody = strings.TrimSuffix(string(message[:]), "\n")
	case contentTypeTextHtml:
		message, _ := ioutil.ReadAll(msg.Body)
		email.HTMLBody = strings.TrimSuffix(string(message[:]), "\n")
	default:
		email.Content, err = decodeContent(msg.Body, msg.Header.Get("Content-Transfer-Encoding"))
	}

	return
}

func createEmailFromHeader(header mail.Header) (email Email, err error) {
	hp := headerParser{header: &header}

	email.Subject = decodeMimeSentence(header.Get("Subject"))
	email.From = hp.parseAddressList(header.Get("From"))
	email.Sender = hp.parseAddress(header.Get("Sender"))
	email.ReplyTo = hp.parseAddressList(header.Get("Reply-To"))
	email.To = hp.parseAddressList(header.Get("To"))
	email.Cc = hp.parseAddressList(header.Get("Cc"))
	email.Bcc = hp.parseAddressList(header.Get("Bcc"))
	email.Date = hp.parseTime(header.Get("Date"))
	email.ResentFrom = hp.parseAddressList(header.Get("Resent-From"))
	email.ResentSender = hp.parseAddress(header.Get("Resent-Sender"))
	email.ResentTo = hp.parseAddressList(header.Get("Resent-To"))
	email.ResentCc = hp.parseAddressList(header.Get("Resent-Cc"))
	email.ResentBcc = hp.parseAddressList(header.Get("Resent-Bcc"))
	email.ResentMessageID = hp.parseMessageId(header.Get("Resent-Message-ID"))
	email.MessageID = hp.parseMessageId(header.Get("Message-ID"))
	email.InReplyTo = hp.parseMessageIdList(header.Get("In-Reply-To"))
	email.References = hp.parseMessageIdList(header.Get("References"))
	email.ResentDate = hp.parseTime(header.Get("Resent-Date"))

	if hp.err != nil {
		err = hp.err
		return
	}

	//decode whole header for easier access to extra fields
	//todo: should we decode? aren't only standard fields mime encoded?
	email.Header, err = decodeHeaderMime(header)
	if err != nil {
		return
	}

	return
}

func parseContentType(contentTypeHeader string) (contentType string, params map[string]string, err error) {
	if contentTypeHeader == "" {
		contentType = contentTypeTextPlain
		return
	}

	return mime.ParseMediaType(contentTypeHeader)
}

func parseMultipartRelated(msg io.Reader, boundary string) (textBody, htmlBody string, embeddedFiles []EmbeddedFile, err error) {
	pmr := multipart.NewReader(msg, boundary)
	for {
		part, err := pmr.NextPart()

		if err == io.EOF {
			break
		} else if err != nil {
			return textBody, htmlBody, embeddedFiles, err
		}

		contentType, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			return textBody, htmlBody, embeddedFiles, err
		}

		switch contentType {
		case contentTypeTextPlain:
			ppContent, err := ioutil.ReadAll(part)
			if err != nil {
				return textBody, htmlBody, embeddedFiles, err
			}

			textBody += strings.TrimSuffix(string(ppContent[:]), "\n")
		case contentTypeTextHtml:
			ppContent, err := ioutil.ReadAll(part)
			if err != nil {
				return textBody, htmlBody, embeddedFiles, err
			}

			htmlBody += strings.TrimSuffix(string(ppContent[:]), "\n")
		case contentTypeMultipartAlternative:
			tb, hb, ef, err := parseMultipartAlternative(part, params["boundary"])
			if err != nil {
				return textBody, htmlBody, embeddedFiles, err
			}

			htmlBody += hb
			textBody += tb
			embeddedFiles = append(embeddedFiles, ef...)
		default:
			if isEmbeddedFile(part) {
				ef, err := decodeEmbeddedFile(part)
				if err != nil {
					return textBody, htmlBody, embeddedFiles, err
				}

				embeddedFiles = append(embeddedFiles, ef)
			} else {
				return textBody, htmlBody, embeddedFiles, fmt.Errorf("Can't process multipart/related inner mime type: %s", contentType)
			}
		}
	}

	return textBody, htmlBody, embeddedFiles, err
}

func parseMultipartAlternative(msg io.Reader, boundary string) (textBody, htmlBody string, embeddedFiles []EmbeddedFile, err error) {
	pmr := multipart.NewReader(msg, boundary)
	for {
		part, err := pmr.NextPart()

		if err == io.EOF {
			break
		} else if err != nil {
			return textBody, htmlBody, embeddedFiles, err
		}

		contentType, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			return textBody, htmlBody, embeddedFiles, err
		}

		switch contentType {
		case contentTypeTextPlain:
			ppContent, err := ioutil.ReadAll(part)
			if err != nil {
				return textBody, htmlBody, embeddedFiles, err
			}

			textBody += strings.TrimSuffix(string(ppContent[:]), "\n")
		case contentTypeTextHtml:
			ppContent, err := ioutil.ReadAll(part)
			if err != nil {
				return textBody, htmlBody, embeddedFiles, err
			}

			htmlBody += strings.TrimSuffix(string(ppContent[:]), "\n")
		case contentTypeMultipartRelated:
			tb, hb, ef, err := parseMultipartRelated(part, params["boundary"])
			if err != nil {
				return textBody, htmlBody, embeddedFiles, err
			}

			htmlBody += hb
			textBody += tb
			embeddedFiles = append(embeddedFiles, ef...)
		default:
			if isEmbeddedFile(part) {
				ef, err := decodeEmbeddedFile(part)
				if err != nil {
					return textBody, htmlBody, embeddedFiles, err
				}

				embeddedFiles = append(embeddedFiles, ef)
			} else {
				return textBody, htmlBody, embeddedFiles, fmt.Errorf("Can't process multipart/alternative inner mime type: %s", contentType)
			}
		}
	}

	return textBody, htmlBody, embeddedFiles, err
}

func parseMultipartMixed(msg io.Reader, boundary string) (textBody, htmlBody string, attachments []Attachment, embeddedFiles []EmbeddedFile, err error) {
	mr := multipart.NewReader(msg, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return textBody, htmlBody, attachments, embeddedFiles, err
		}

		contentType, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			return textBody, htmlBody, attachments, embeddedFiles, err
		}

		if contentType == contentTypeMultipartAlternative {
			textBody, htmlBody, embeddedFiles, err = parseMultipartAlternative(part, params["boundary"])
			if err != nil {
				return textBody, htmlBody, attachments, embeddedFiles, err
			}
		} else if contentType == contentTypeMultipartRelated {
			textBody, htmlBody, embeddedFiles, err = parseMultipartRelated(part, params["boundary"])
			if err != nil {
				return textBody, htmlBody, attachments, embeddedFiles, err
			}
		} else if contentType == contentTypeTextPlain {
			ppContent, err := ioutil.ReadAll(part)
			if err != nil {
				return textBody, htmlBody, attachments, embeddedFiles, err
			}

			textBody += strings.TrimSuffix(string(ppContent[:]), "\n")
		} else if contentType == contentTypeTextHtml {
			ppContent, err := ioutil.ReadAll(part)
			if err != nil {
				return textBody, htmlBody, attachments, embeddedFiles, err
			}

			htmlBody += strings.TrimSuffix(string(ppContent[:]), "\n")
		} else if isAttachment(part) {
			at, err := decodeAttachment(part)
			if err != nil {
				return textBody, htmlBody, attachments, embeddedFiles, err
			}

			attachments = append(attachments, at)
		} else {
			return textBody, htmlBody, attachments, embeddedFiles, fmt.Errorf("Unknown multipart/mixed nested mime type: %s", contentType)
		}
	}

	return textBody, htmlBody, attachments, embeddedFiles, err
}

func decodeMimeSentence(s string) string {
	result := []string{}
	ss := strings.Split(s, " ")

	for _, word := range ss {
		dec := new(mime.WordDecoder)
		w, err := dec.Decode(word)
		if err != nil {
			if len(result) == 0 {
				w = word
			} else {
				w = " " + word
			}
		}

		result = append(result, w)
	}

	return strings.Join(result, "")
}

func decodeHeaderMime(header mail.Header) (mail.Header, error) {
	parsedHeader := map[string][]string{}

	for headerName, headerData := range header {

		parsedHeaderData := []string{}
		for _, headerValue := range headerData {
			parsedHeaderData = append(parsedHeaderData, decodeMimeSentence(headerValue))
		}

		parsedHeader[headerName] = parsedHeaderData
	}

	return mail.Header(parsedHeader), nil
}

func isEmbeddedFile(part *multipart.Part) bool {
	return part.Header.Get("Content-Transfer-Encoding") != ""
}

func decodeEmbeddedFile(part *multipart.Part) (ef EmbeddedFile, err error) {
	cid := decodeMimeSentence(part.Header.Get("Content-Id"))
	decoded, err := decodeContent(part, part.Header.Get("Content-Transfer-Encoding"))
	if err != nil {
		return
	}

	ef.CID = strings.Trim(cid, "<>")
	ef.Data = decoded
	ef.ContentType = part.Header.Get("Content-Type")

	return
}

func isAttachment(part *multipart.Part) bool {
	return part.FileName() != ""
}

func decodeAttachment(part *multipart.Part) (at Attachment, err error) {
	//-sanek
	//+sanek
	filename := ""
	s1 := part.Header.Get("Content-Disposition")
	s1 = strings.ReplaceAll(s1, "attachment; filename=", "")

	////s1 := part.Header.Get("Content-Type")
	//s_low := strings.ToLower(s1)
	//if len(s_low) > 42 && s_low[0:42] == "application/vnd.ms-excel; name=\"=?utf-8?b?" {
	//	s2 := s1[42:]
	//	filename = FindFilenameFromBase64(s2, "?= =?UTF-8?B?")
	//} else if len(s_low) > 49 && s_low[0:49] == "application/vnd.ms-excel; name=\"=?windows-1251?b?" {
	//	s2 := s1[49:]
	//	s3 := FindFilenameFromBase64(s2, "?= =?windows-1251?B?")
	//	//s3, err := b64.StdEncoding.DecodeString(s2)
	//	s4 := DecodeWindows1251([]byte(s3))
	//	//if err == nil {
	//	filename = string(s4)
	//	//}
	//
	//} else if len(s_low) > 83 && s_low[0:83] == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet; name=\"=?utf-8?b?" {
	//	s2 := s1[83:]
	//	filename = FindFilenameFromBase64(s2, "?= =?UTF-8?B?")
	//
	//} else {
	//	filename = decodeMimeSentence(part.FileName())
	//}
	filename = FindFilenameFromAttachment(s1)
	filename = strings.Replace(filename, ":", "_", -1)
	filename = strings.Replace(filename, "\t", " ", -1)
	filename = strings.ReplaceAll(filename, "\"", "")

	if filename == "" {
		filename = decodeMimeSentence(part.FileName())
	}
	//filename = decodeMimeSentence(part.FileName())
	decoded, err := decodeContent(part, part.Header.Get("Content-Transfer-Encoding"))
	if err != nil {
		return
	}

	at.Filename = filename
	at.Data = decoded
	at.ContentType = strings.Split(part.Header.Get("Content-Type"), ";")[0]

	return
}

func decodeContent(content io.Reader, encoding string) (io.Reader, error) {
	switch encoding {
	case "base64":
		decoded := base64.NewDecoder(base64.StdEncoding, content)
		b, err := ioutil.ReadAll(decoded)
		if err != nil {
			return nil, err
		}

		return bytes.NewReader(b), nil
	case "7bit":
		dd, err := ioutil.ReadAll(content)
		if err != nil {
			return nil, err
		}

		return bytes.NewReader(dd), nil
	case "":
		return content, nil
	default:
		return nil, fmt.Errorf("unknown encoding: %s", encoding)
	}
}

type headerParser struct {
	header *mail.Header
	err    error
}

func (hp headerParser) parseAddress(s string) (ma *mail.Address) {
	if hp.err != nil {
		return nil
	}

	if strings.Trim(s, " \n") != "" {
		ma, hp.err = mail.ParseAddress(s)

		return ma
	}

	return nil
}

func (hp headerParser) parseAddressList(s string) (ma []*mail.Address) {
	if hp.err != nil {
		return
	}

	if strings.Trim(s, " \n") != "" {
		ma, hp.err = mail.ParseAddressList(s)
		return
	}

	return
}

func (hp headerParser) parseTime(s string) (t time.Time) {
	if hp.err != nil || s == "" {
		return
	}

	formats := []string{
		time.RFC1123Z,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		time.RFC1123Z + " (MST)",
		"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
	}

	for _, format := range formats {
		t, hp.err = time.Parse(format, s)
		if hp.err == nil {
			return
		}
	}

	return
}

func (hp headerParser) parseMessageId(s string) string {
	if hp.err != nil {
		return ""
	}

	return strings.Trim(s, "<> ")
}

func (hp headerParser) parseMessageIdList(s string) (result []string) {
	if hp.err != nil {
		return
	}

	for _, p := range strings.Split(s, " ") {
		if strings.Trim(p, " \n") != "" {
			result = append(result, hp.parseMessageId(p))
		}
	}

	return
}

// Attachment with filename, content type and data (as a io.Reader)
type Attachment struct {
	Filename    string
	ContentType string
	Data        io.Reader
}

// EmbeddedFile with content id, content type and data (as a io.Reader)
type EmbeddedFile struct {
	CID         string
	ContentType string
	Data        io.Reader
}

// Email with fields for all the headers defined in RFC5322 with it's attachments and
type Email struct {
	Header mail.Header

	Subject    string
	Sender     *mail.Address
	From       []*mail.Address
	ReplyTo    []*mail.Address
	To         []*mail.Address
	Cc         []*mail.Address
	Bcc        []*mail.Address
	Date       time.Time
	MessageID  string
	InReplyTo  []string
	References []string

	ResentFrom      []*mail.Address
	ResentSender    *mail.Address
	ResentTo        []*mail.Address
	ResentDate      time.Time
	ResentCc        []*mail.Address
	ResentBcc       []*mail.Address
	ResentMessageID string

	ContentType string
	Content     io.Reader

	HTMLBody string
	TextBody string

	Attachments   []Attachment
	EmbeddedFiles []EmbeddedFile
}

////Sanek
//// FileName returns the filename parameter of the Part's Content-Disposition
//// header. If not empty, the filename is passed through filepath.Base (which is
//// platform dependent) before being returned.
//func FileName(p *multipart.Part) string {
//	if p.dispositionParams == nil {
//		p.parseContentDisposition()
//	}
//	filename := p.dispositionParams["filename"]
//	if filename == "" {
//		return ""
//	}
//
//	if filename[0:9] == "=?utf-8?B?" {
//		sDec, err := b64.StdEncoding.DecodeString(filename[10:])
//		if err != nil {
//			filename = string(sDec)
//			return filename
//		}
//	}
//
//	// RFC 7578, Section 4.2 requires that if a filename is provided, the
//	// directory path information must not be used.
//	return filepath.Base(filename)
//}

func DecodeWindows1251(ba []uint8) []uint8 {
	dec := charmap.Windows1251.NewDecoder()
	out, _ := dec.Bytes(ba)
	return out
}

func EncodeWindows1251(ba []uint8) []uint8 {
	enc := charmap.Windows1251.NewEncoder()
	out, _ := enc.String(string(ba))
	return []uint8(out)
}

func FindFilenameFromBase64(s string, Separator string) string {
	Otvet := ""

	if len(s) > 4 && s[len(s)-3:] == "?=\"" {
		s = s[:len(s)-3]
	}

	MassS := SplitCaseInsensivity(s, Separator)
	for _, Mass1 := range MassS {
		sDec, err := b64.StdEncoding.DecodeString(Mass1)
		if err == nil {
			Otvet = Otvet + string(sDec)
		}
	}

	return Otvet
}

func SplitCaseInsensivity(s, Separator string) []string {
	var Otvet []string

	//s2 := strings.ToLower(s)
	Separator = strings.ToLower(Separator)
	LenSeparator := len(Separator)

	s3 := s

	var SNow string
	f := 0
	StartFrom := 0
	for (f + LenSeparator) < len(s3) {
		SNow = s[StartFrom+f : StartFrom+f+LenSeparator]
		if (f+LenSeparator) < len(s3) && strings.ToLower(SNow) == Separator {
			Otvet = append(Otvet, s[StartFrom:StartFrom+f])
			s3 = s[StartFrom+f+LenSeparator:]
			StartFrom = StartFrom + f + LenSeparator
			f = -1
		}
		f = f + 1
	}

	if s3 != "" {
		Otvet = append(Otvet, s3)
	}

	return Otvet
}

func FindFilenameFromAttachment(s string) string {
	Otvet := ""

	var MassS []string
	MassS = make([]string, 0)
	LenS := len(s)

	if s[LenS-1:LenS] == "\"" {
		s = s[:LenS-1]
	}

	if s[0:1] == "\"" {
		s = s[1:]
	}
	LenS = len(s)

	Start := 1
	s2 := ""
	var s1 string
	for i := 1; i < (LenS - 3); i++ {

		s1 = s[i-1 : i+2]
		if s1 == " =?" {
			s2 = s[Start-1 : i-1]
			if s2 != "" {
				MassS = append(MassS, s2)
			}
			Start = i + 3
		}
	}

	if Start < LenS {
		s2 = s[Start-1:]
		if s2 != "" {
			MassS = append(MassS, s2)
		}
	}

	//уберём в конце ?=
	for f, Mass1 := range MassS {
		pos1 := strings.Index(Mass1, "?=")
		if pos1 > 0 {
			MassS[f] = Mass1[:pos1]
		}

		if Mass1[0:2] == "=?" {
			MassS[f] = Mass1[2:]
		}
		//len1 := len(Mass1)
		//if len1 > 3 && Mass1[len1-3:] == "?=\"" {
		//	MassS[f] = Mass1[:len1-3]
		//}
		//if len1 > 2 && Mass1[len1-2:] == "?=" {
		//	MassS[f] = Mass1[:len1-2]
		//}
	}

	//
	for _, Mass1 := range MassS {
		len1 := len(Mass1)
		if len1 > 15 && strings.ToLower(Mass1[:15]) == "windows-1251?b?" {
			s2 = Mass1[15:]
			s3, err := b64.StdEncoding.DecodeString(s2)
			s4 := DecodeWindows1251([]byte(s3))
			if err == nil {
				Otvet = Otvet + string(s4)
			}
		} else if len1 > 8 && strings.ToLower(Mass1[:8]) == "utf-8?b?" {
			s2 = Mass1[8:]
			s4 := ""

			pos1 := strings.Index(s2, "?=")
			if pos1 > 0 {
				s4 = s2[pos1+2:]
				s2 = s2[:pos1]
			}

			s3, err := b64.StdEncoding.DecodeString(s2)
			if err == nil {
				Otvet = Otvet + string(s3) + s4
			}
		} else if len1 > 9 && strings.ToLower(Mass1[:9]) == "koi8-r?b?" {
			s2 = Mass1[9:]
			s3, err := b64.StdEncoding.DecodeString(s2)
			if err == nil {
				d := charmap.KOI8R.NewDecoder()
				Otvet1, _ := d.Bytes([]byte(s3))
				Otvet = Otvet + string(Otvet1)
			}
		} else if len1 > 15 && strings.ToLower(Mass1[:15]) == "windows-1251?q?" {
			s2 = Mass1[15:]

			Otvet1, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(s2)))
			if err == nil {
				Otvet1 = DecodeWindows1251([]byte(Otvet1))
				Otvet = Otvet + string(Otvet1)
			}

		} else if len1 > 8 && strings.ToLower(Mass1[:8]) == "utf-8?q?" {
			s2 = Mass1[8:]

			Otvet1, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(s2)))
			if err == nil {
				Otvet = Otvet + string(Otvet1)
			}
		} else if len1 > 9 && strings.ToLower(Mass1[:9]) == "koi8-r?q?" {
			s2 = Mass1[9:]
			s3, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(s2)))
			if err == nil {
				d := charmap.KOI8R.NewDecoder()
				Otvet1, _ := d.Bytes([]byte(s3))
				Otvet = Otvet + string(Otvet1)
			}
		}
	}

	return Otvet
}
