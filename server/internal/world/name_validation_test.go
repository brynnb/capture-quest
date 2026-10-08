package world

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/testdb"
)

func TestNameValidationUsesOwnedLookupAndNeverConvertsSQLFailureToAvailable(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `UPDATE character_data SET name='Battle' WHERE id=42`)
	db.GlobalWorldDB = nil
	for _, test := range []struct {
		name      string
		available bool
	}{{"Freshname", true}, {"Battle", false}} {
		HandleValidateNameRequest(ses, []byte(`{"requestId":"name:check","name":"`+test.name+`"}`), wh)
		var result protocol.NameValidationResponse
		if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &result); err != nil || !result.Success || !result.Valid || result.Available != test.available || result.Name != test.name || result.RequestID != "name:check" {
			t.Fatalf("validation=%+v error=%v", result, err)
		}
	}
	testdb.Exec(t, database, `UPDATE character_data SET deleted_at=CURRENT_TIMESTAMP WHERE id=42`)
	HandleValidateNameRequest(ses, []byte(`{"requestId":"name:deleted","name":"Battle"}`), wh)
	var deleted protocol.NameValidationResponse
	json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &deleted)
	if !deleted.Success || deleted.Available {
		t.Fatal("soft-deleted UNIQUE name became available")
	}
	testdb.Exec(t, database, `ALTER TABLE character_data RENAME TO unavailable_characters`)
	HandleValidateNameRequest(ses, []byte(`{"requestId":"name:failed","name":"Freshname"}`), wh)
	var failure protocol.NameValidationError
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failure); err != nil || failure.Success || failure.RequestID != "name:failed" || failure.Error == "" {
		t.Fatalf("SQL failure=%+v error=%v", failure, err)
	}
}

func TestNameValidationPoolWaitHonorsCallerCancellation(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `UPDATE character_data SET name='Battle' WHERE id=42`)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ses.ExecuteCommand(ctx, func() { HandleValidateNameRequest(ses, []byte(`{"requestId":"name:cancel","name":"Freshname"}`), wh) }); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.ValidateNameResponse {
		t.Fatal("cancelled validation omitted response")
	}
	var failure protocol.NameValidationError
	if err := json.Unmarshal(messages.streams[0].payload, &failure); err != nil || failure.Success || failure.RequestID != "name:cancel" || failure.Error == "" {
		t.Fatalf("cancelled validation=%+v error=%v", failure, err)
	}
}

func TestNameFilterFailureRejectsValidationAndCreation(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `DROP TABLE disallowed_words`)
	HandleValidateNameRequest(ses, []byte(`{"requestId":"name:filter","name":"Freshname"}`), wh)
	var failure protocol.NameValidationError
	if err := json.Unmarshal(messages.streams[0].payload, &failure); err != nil || failure.Success || failure.Error == "" || failure.RequestID != "name:filter" {
		t.Fatalf("filter failure=%+v %v", failure, err)
	}
	HandleCharacterCreate(ses, []byte(`{"name":"Freshname"}`), wh)
	var result SimpleSuccessResponse
	if len(messages.streams) != 2 || json.Unmarshal(messages.streams[1].payload, &result) != nil || result.Value != 0 {
		t.Fatal("filter failure allowed creation or omitted reply")
	}
}
