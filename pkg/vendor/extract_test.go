package vendor

import (
	"strings"
	"testing"
)

func TestParseTcPOU(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<TcPlcObject Version="1.1.0.1" ProductVersion="3.1.4024.6">
  <POU Name="FB_MyBlock" Id="{12345}" SpecialFunc="None">
    <Declaration><![CDATA[
FUNCTION_BLOCK FB_MyBlock
VAR_INPUT
    bExecute : BOOL;
    nValue   : INT;
END_VAR
VAR_OUTPUT
    bDone  : BOOL;
    bError : BOOL;
END_VAR
    ]]></Declaration>
    <Implementation>
      <ST><![CDATA[
IF bExecute THEN
    bDone := TRUE;
END_IF;
      ]]></ST>
    </Implementation>
  </POU>
</TcPlcObject>`

	pou, err := ParseTcPOU(strings.NewReader(xml))
	if err != nil {
		t.Fatalf("ParseTcPOU failed: %v", err)
	}

	if pou.Name != "FB_MyBlock" {
		t.Errorf("expected Name 'FB_MyBlock', got %q", pou.Name)
	}

	if !strings.Contains(pou.Declaration, "FUNCTION_BLOCK FB_MyBlock") {
		t.Errorf("declaration should contain FUNCTION_BLOCK FB_MyBlock, got %q", pou.Declaration)
	}

	if !strings.Contains(pou.Declaration, "bExecute : BOOL") {
		t.Errorf("declaration should contain 'bExecute : BOOL', got %q", pou.Declaration)
	}

	// Implementation should be captured but not used for stubs
	if !strings.Contains(pou.Implementation.ST, "bDone := TRUE") {
		t.Errorf("implementation should contain body, got %q", pou.Implementation.ST)
	}
}

func TestExtractStub(t *testing.T) {
	pou := &TcPOU{
		Name: "FB_Test",
		Declaration: `FUNCTION_BLOCK FB_Test
VAR_INPUT
    x : INT;
END_VAR
VAR_OUTPUT
    y : INT;
END_VAR`,
		Implementation: TcImplementation{ST: "y := x * 2;"},
	}

	stub := ExtractStub(pou)
	if !strings.Contains(stub, "FUNCTION_BLOCK FB_Test") {
		t.Errorf("stub should contain declaration, got %q", stub)
	}
	if strings.Contains(stub, "y := x * 2") {
		t.Error("stub should NOT contain implementation body")
	}
}

func TestExtractStubEmpty(t *testing.T) {
	pou := &TcPOU{
		Name:        "Empty",
		Declaration: "",
	}
	stub := ExtractStub(pou)
	if stub != "" {
		t.Errorf("expected empty stub for empty declaration, got %q", stub)
	}
}

func TestParsePlcProj(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<Project DefaultTargets="Build" xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <ItemGroup>
    <Compile Include="POUs\FB_Motor.TcPOU" />
    <Compile Include="POUs\FB_Conveyor.TcPOU" />
    <Compile Include="GVLs\GVL_IO.TcGVL" />
    <Compile Include="DUTs\ST_Config.TcDUT" />
  </ItemGroup>
  <ItemGroup>
    <Compile Include="POUs\FB_Safety.TcPOU" />
  </ItemGroup>
</Project>`

	pouFiles, err := ParsePlcProj(strings.NewReader(xml))
	if err != nil {
		t.Fatalf("ParsePlcProj failed: %v", err)
	}

	if len(pouFiles) != 3 {
		t.Fatalf("expected 3 .TcPOU files, got %d: %v", len(pouFiles), pouFiles)
	}

	expected := []string{
		`POUs\FB_Motor.TcPOU`,
		`POUs\FB_Conveyor.TcPOU`,
		`POUs\FB_Safety.TcPOU`,
	}

	for i, exp := range expected {
		if pouFiles[i] != exp {
			t.Errorf("pouFiles[%d]: expected %q, got %q", i, exp, pouFiles[i])
		}
	}
}

func TestParseTcPOUInvalid(t *testing.T) {
	_, err := ParseTcPOU(strings.NewReader("not xml"))
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}
