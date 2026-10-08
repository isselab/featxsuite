package ftx

import "testing"

func TestFormatNormalisesSpacing(t *testing.T) {
	src := "feature_model:    M\n  xor nav   ?  :   static    early\n      child:dynamic late\n"

	got, ok := Format(Parse(src))
	if !ok {
		t.Fatalf("Format refused: %v", messages(Parse(src).Diagnostics))
	}

	want := "feature_model: M\n" +
		"    xor nav ?: static early\n" +
		"        child : dynamic late\n"

	if got != want {
		t.Errorf("Format() =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatIsIdempotent(t *testing.T) {
	once, ok := Format(Parse(modelSrc))
	if !ok {
		t.Fatal("Format refused the project's own model")
	}

	twice, ok := Format(Parse(once))
	if !ok {
		t.Fatal("Format refused its own output")
	}

	if once != twice {
		t.Errorf("Format is not idempotent:\nfirst:\n%s\nsecond:\n%s", once, twice)
	}
}

func TestFormatKeepsTheRealModelStable(t *testing.T) {
	// The canonical form is what features.ftx already uses, so formatting it
	// should be a no-op.
	got, ok := Format(Parse(modelSrc))
	if !ok {
		t.Fatal("Format refused the project's own model")
	}

	if got != modelSrc {
		t.Errorf("formatting features.ftx changed it:\ngot:\n%s\nwant:\n%s", got, modelSrc)
	}
}

func TestFormatKeepsTheRealConfigStable(t *testing.T) {
	got, ok := Format(Parse(configSrc))
	if !ok {
		t.Fatal("Format refused the project's own configuration")
	}

	if got != configSrc {
		t.Errorf("formatting configs.ftx changed it:\ngot:\n%s\nwant:\n%s", got, configSrc)
	}
}

func TestFormatRefusesBrokenDocuments(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"no header", "nav: static early\n"},
		{"missing binding", "feature_model: M\n    nav: static\n"},
		{"missing colon", "feature_model: M\n    nav static early\n"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := Format(Parse(tc.src)); ok {
				t.Error("Format accepted a document with syntax errors")
			}
		})
	}
}
