package main

import (
	"reflect"
	"testing"
)

func TestParseLeadingFlags(t *testing.T) {
	g, rest, err := parseLeadingFlags([]string{"--log-level", "debug", "init", "--foo"})
	if err != nil {
		t.Fatal(err)
	}
	if g.LogLevel != "debug" {
		t.Fatalf("LogLevel=%q", g.LogLevel)
	}
	if !reflect.DeepEqual(rest, []string{"init", "--foo"}) {
		t.Fatalf("rest=%v", rest)
	}
}

func TestParseLeadingFlags_configEquals(t *testing.T) {
	g, rest, err := parseLeadingFlags([]string{"--config=/x.yaml", "run"})
	if err != nil {
		t.Fatal(err)
	}
	if g.ConfigPath != "/x.yaml" || len(rest) != 1 || rest[0] != "run" {
		t.Fatalf("g=%+v rest=%v", g, rest)
	}
}

func TestParseLeadingFlags_doubleDash(t *testing.T) {
	_, rest, err := parseLeadingFlags([]string{"--", "init", "-x"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rest, []string{"init", "-x"}) {
		t.Fatalf("rest=%v", rest)
	}
}
