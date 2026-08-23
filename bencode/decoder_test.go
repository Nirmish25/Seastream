package bencode

import "testing"

func TestDecodeRejectsNegativeStringLength(t *testing.T) {
	if _, err := Decode([]byte("-1:a")); err == nil {
		t.Fatal("expected negative string length error")
	}
}

func TestValueRangeReturnsRawInfoBytes(t *testing.T) {
	data := []byte("d4:infod4:name4:spame4:otheri1ee")
	start, end, err := ValueRange(data, "info")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data[start:end]), "d4:name4:spame"; got != want {
		t.Fatalf("ValueRange() = %q, want %q", got, want)
	}
}
