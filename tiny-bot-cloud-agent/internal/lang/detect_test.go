package lang

import "testing"

func TestDetect_English(t *testing.T) {
	cases := []string{
		"How is the weather today?",
		"Hello, nice to meet you!",
		"OK",
	}
	for _, s := range cases {
		if got := Detect(s); got != En {
			t.Errorf("Detect(%q) = %q, want en", s, got)
		}
	}
}

func TestDetect_Cantonese(t *testing.T) {
	cases := []string{
		"今日天气点呀？",
		"你喺边度呀？",
		"佢唔知几时嚟。",
		"做咩咁耐都未到？",
	}
	for _, s := range cases {
		if got := Detect(s); got != Yue {
			t.Errorf("Detect(%q) = %q, want yue", s, got)
		}
	}
}

func TestDetect_Mandarin(t *testing.T) {
	cases := []string{
		"你好，今天天气怎么样？",
		"我想听一首歌。",
		"现在几点了？",
	}
	for _, s := range cases {
		if got := Detect(s); got != Zh {
			t.Errorf("Detect(%q) = %q, want zh", s, got)
		}
	}
}

func TestDetect_Empty(t *testing.T) {
	if got := Detect(""); got != Zh {
		t.Errorf("Detect(\"\") = %q, want zh", got)
	}
}

func TestCode_Label(t *testing.T) {
	if Zh.Label() != "普通话" {
		t.Fatal(Zh.Label())
	}
	if Yue.Label() != "粤语" {
		t.Fatal(Yue.Label())
	}
	if En.Label() != "English" {
		t.Fatal(En.Label())
	}
}
