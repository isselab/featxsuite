package ftx

import "testing"

// Every validator form has to survive formatting, including a parameter
// carrying several.
func TestFormatPreservesEveryValidatorForm(t *testing.T) {
	src := `RB-1-BASE:
    demo : node {dynamic late}
        a : double (1.5) bounds<0.0, 3.0>
        b : double (1.0) gt<0.0>
        c : double (1.0) gt_eq<0.0>
        d : double (1.0) lt<3.0>
        e : double (3.0) lt_eq<3.0>
        f : string (fast) one_of<slow, fast>
        g : double_array (0.0, 0.0) fixed_size<2>
        h : string (autonomous) size_gt<2>
        i : string (autonomous) size_lt<20>
        j : string (autonomous) not_empty<>
        k : string (autonomous) not_empty<> size_gt<2> size_lt<20>
`

	out, _ := Format(Parse(src))
	if out != src {
		t.Errorf("formatting changed the source:\n--- got ---\n%s--- want ---\n%s", out, src)
	}

	again, _ := Format(Parse(out))
	if again != out {
		t.Errorf("formatting is not stable:\n--- second ---\n%s--- first ---\n%s", again, out)
	}
}

// Where a validator sits is the author's choice, so formatting keeps it: one
// on the parameter's line stays there, and one written beneath stays beneath.
func TestFormatKeepsValidatorsWhereTheyWereWritten(t *testing.T) {
	src := `RB-1-BASE:
    demo : node {dynamic late}
        inline : double (1.5) bounds<0.0, 3.0>
        beneath : string ("autonomous")
            one_of<"autonomous", "manual">
        several : string (autonomous)
            not_empty<>
            size_gt<2>
        shared : double (1.5)
            gt<0.0> lt<3.0>
        both : double (1.5) gt<0.0>
            lt<3.0>
`

	out, _ := Format(Parse(src))
	if out != src {
		t.Errorf("formatting moved a validator:\n--- got ---\n%s--- want ---\n%s", out, src)
	}

	again, _ := Format(Parse(out))
	if again != out {
		t.Errorf("formatting is not stable:\n--- second ---\n%s", again)
	}
}
