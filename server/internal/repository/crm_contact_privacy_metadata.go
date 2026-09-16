package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// These are structured identity fields, not free-text detection. In particular,
// message/comment bodies, subjects, and their quoted text intentionally remain.
func redactContactIdentityJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode identity metadata: %w", err)
	}
	keyNormalizer := strings.NewReplacer("_", "", "-", "")
	var redact func(any)
	redact = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			for key, child := range node {
				normalized := strings.ToLower(keyNormalizer.Replace(key))
				switch normalized {
				case "name", "firstname", "lastname", "displayname", "customername", "sendername", "senderdisplayname", "fromdisplayname",
					"originalsenderemail", "originalsendername", "forwardedbyemail", "forwardedbyname",
					"email", "emailaddress", "customeremail", "senderemail", "oldemail", "newemail", "phone", "phonenumber", "customerphone",
					"ip", "ipaddress", "useragent", "anonymousid", "externaluserid", "contactid", "crmcontactid", "avatarurl", "senderavatarurl",
					"searchvector", "viewsearchdocument", "lastpublicsenderdisplayname", "lastmessagesenderdisplayname",
					"suggestedprimaryrecipientemail", "suggestedprimaryrecipientname", "emailthreadparticipants", "visitorcountrycode", "visitorcountryname",
					"sessiontoken", "identitysignature", "signature", "lastpageurl", "countrycode", "countryname", "regionname", "cityname",
					"from", "to", "cc", "bcc", "replyto", "originalrecipient", "recipientaddress", "recipientemail", "deliverytoemail", "toemail", "fromemail", "emailcc", "emailbcc", "ccemails", "bccemails", "recipientname", "fromfull", "tofull", "ccfull", "bccfull", "headers", "rfcmessageid":
					delete(node, key)
				default:
					redact(child)
				}
			}
		case []any:
			for _, child := range node {
				redact(child)
			}
		}
	}
	redact(value)
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode identity metadata: %w", err)
	}
	return data, nil
}

// Table and column are fixed internal identifiers; the scope contains only bound values.
func redactContactMetadata(scope *gorm.DB, table, column string) error {
	var rows []struct {
		ID   string
		Data json.RawMessage
	}
	return scope.Session(&gorm.Session{}).Table(table).Select("id, "+column+" AS data").FindInBatches(&rows, 100, func(_ *gorm.DB, _ int) error {
		for _, row := range rows {
			cleaned, err := redactContactIdentityJSON(row.Data)
			if err != nil {
				return err
			}
			if err := scope.Session(&gorm.Session{}).Table(table).Where("id = ?", row.ID).Update(column, string(cleaned)).Error; err != nil {
				return err
			}
		}
		return nil
	}).Error
}
