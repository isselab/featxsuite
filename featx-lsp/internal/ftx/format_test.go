package ftx

import "testing"

// Every validator form has to survive formatting. Rebuilding one from its
// arguments alone dropped the `#`, so `<0.0, #>` came back as `<0.0>` and read
// as a fixed size: a silent change of meaning on format.
func TestFormatPreservesEveryValidatorForm(t *testing.T) {
	src := `RB-1-BASE:
    demo : node {dynamic late}
        a : double (1.0) <0.0, #>
        b : double (1.0) <#, 3.0>
        c : double (1.0) <!0.0, #>
        d : double (1.0) <#, !3.0>
        e : double (1.5) <!0.0, !3.0>
        f : double (1.5) <!0.0, 3.0>
        g : double (1.5) <0.0, !3.0>
        h : double (1.5) <0.0, 3.0>
        i : double_array (0.0) <4>
        j : string (fast) <<slow, fast>>
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
