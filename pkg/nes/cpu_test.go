package nes

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/cpu"
	. "github.com/retroenv/retrogolib/addressing"
	"github.com/retroenv/retrogolib/arch/nes"
	"github.com/retroenv/retrogolib/assert"
)

type cpuTest struct {
	Name  string
	Setup func(sys *System)
	Check func(sys *System)
}

func runCPUTest(t *testing.T, tests []cpuTest) {
	t.Helper()

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			sys := NewSystem(nil)
			test.Setup(sys)
			test.Check(sys)
		})
	}
}

func TestAdc(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "result 0x00",
			Setup: func(sys *System) {
				sys.A = 2
				sys.Adc(0xff)
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
			},
		},
		{
			Name: "result 0x01",
			Setup: func(sys *System) {
				sys.Adc(1)
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.A)
				assert.Equal(t, 0, sys.Flags.C)
			},
		},
		{
			Name: "result 0x102",
			Setup: func(sys *System) {
				sys.A = 2
				sys.Flags.C = 1
				sys.Adc(0xff)
			},
			Check: func(sys *System) {
				assert.Equal(t, 2, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestAnd(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.A = 0x12
	sys.And(2)

	assert.Equal(t, 2, sys.A)
}

func TestAsl(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.A = 0b0000_0001
	sys.Asl()
	assert.Equal(t, 0b0000_0010, sys.A)
	assert.Equal(t, 0, sys.Flags.C)

	sys.A = 0b1111_1110
	sys.Asl()
	assert.Equal(t, 0b1111_1100, sys.A)
	assert.Equal(t, 1, sys.Flags.C)

	sys.Bus.Memory.Write(1, 0b0000_0010)
	sys.Asl(Absolute(1))
	assert.Equal(t, 0b0000_0100, sys.Bus.Memory.Read(1))

	sys.Bus.Memory.Write(4, 0b0000_0010)
	sys.X = 3
	sys.Asl(Absolute(1), sys.X)
	assert.Equal(t, 0b0000_0100, sys.Bus.Memory.Read(4))
}

func TestBcc(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	assert.Equal(t, true, sys.Bcc())

	sys.Flags.C = 1
	assert.Equal(t, false, sys.Bcc())
}

func TestBcs(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	assert.Equal(t, false, sys.Bcs())

	sys.Flags.C = 1
	assert.Equal(t, true, sys.Bcs())
}

func TestBeq(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	assert.Equal(t, false, sys.Beq())

	sys.Flags.Z = 1
	assert.Equal(t, true, sys.Beq())
}

func TestBit(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "value 1",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x100, 1)
				sys.A = 1
				sys.Bit(Absolute(0x100))
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.A)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.V)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
		{
			Name: "value 0xff",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x100, 0xff)
				sys.A = 0xf0
				sys.Bit(Absolute(0x100))
			},
			Check: func(sys *System) {
				assert.Equal(t, 0xf0, sys.A)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 1, sys.Flags.V)
				assert.Equal(t, 1, sys.Flags.N)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestBmi(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	assert.Equal(t, false, sys.Bmi())

	sys.Flags.N = 1
	assert.Equal(t, true, sys.Bmi())
}

func TestBne(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	assert.Equal(t, true, sys.Bne())

	sys.Flags.Z = 1
	assert.Equal(t, false, sys.Bne())
}

func TestBpl(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	assert.Equal(t, true, sys.Bpl())

	sys.Flags.N = 1
	assert.Equal(t, false, sys.Bpl())
}

func TestBrk(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	called := false
	sys.IrqHandler = func() {
		called = true
	}
	sys.Brk()

	assert.True(t, called)
}

func TestBvc(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	assert.Equal(t, true, sys.Bvc())

	sys.Flags.V = 1
	assert.Equal(t, false, sys.Bvc())
}

func TestBvs(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	assert.Equal(t, false, sys.Bvs())

	sys.Flags.V = 1
	assert.Equal(t, true, sys.Bvs())
}

func TestClc(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Flags.C = 1
	sys.Clc()

	assert.Equal(t, 0, sys.Flags.C)
}

func TestCld(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Flags.D = 1
	sys.Cld()

	assert.Equal(t, 0, sys.Flags.D)
}

func TestCli(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Flags.I = 1
	sys.Cli()

	assert.Equal(t, 0, sys.Flags.I)
}

func TestClv(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Flags.V = 1
	sys.Clv()

	assert.Equal(t, 0, sys.Flags.V)
}

func TestCmp(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "equal immediate",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x100, 1)
				sys.A = 1
				sys.Cmp(1)
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
				assert.Equal(t, 1, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
		{
			Name: "unequal absolute",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x100, 0xff)
				sys.A = 1
				sys.Cmp(Absolute(0x100))
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.A)
				assert.Equal(t, 0, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestCpx(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "equal immediate",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x100, 1)
				sys.X = 1
				sys.Cpx(1)
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.X)
				assert.Equal(t, 1, sys.Flags.C)
				assert.Equal(t, 1, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
		{
			Name: "unequal absolute",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x100, 0xff)
				sys.X = 1
				sys.Cpx(Absolute(0x100))
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.X)
				assert.Equal(t, 0, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestCpy(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "equal immediate",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x100, 1)
				sys.Y = 1
				sys.Cpy(1)
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.Y)
				assert.Equal(t, 1, sys.Flags.C)
				assert.Equal(t, 1, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
		{
			Name: "unequal absolute",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x100, 0xff)
				sys.Y = 1
				sys.Cpy(Absolute(0x100))
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.Y)
				assert.Equal(t, 0, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestDec(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "zeropage",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(1, 2)
				sys.Dec(ZeroPage(1))
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.Bus.Memory.Read(1))
			},
		},
		{
			Name: "zeropage x",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(2, 2)
				sys.X = 1
				sys.Dec(ZeroPage(1), &sys.X)
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.Bus.Memory.Read(2))
			},
		},
		{
			Name: "absolute",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x101, 2)
				sys.Dec(Absolute(0x101))
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.Bus.Memory.Read(0x101))
			},
		},
		{
			Name: "absolute x",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x102, 2)
				sys.X = 1
				sys.Dec(Absolute(0x101), &sys.X)
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.Bus.Memory.Read(0x102))
			},
		},
	}
	runCPUTest(t, tests)
}

func TestDex(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.X = 2
	sys.Dex()

	assert.Equal(t, 1, sys.X)
}

func TestDey(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Y = 2
	sys.Dey()

	assert.Equal(t, 1, sys.Y)
}

func TestEor(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	// TODO add test
	sys.Eor(0)
}

func TestInc(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "zeropage",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(1, 1)
				sys.Inc(ZeroPage(1))
			},
			Check: func(sys *System) {
				assert.Equal(t, 2, sys.Bus.Memory.Read(1))
			},
		},
		{
			Name: "zeropage x",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(2, 1)
				sys.X = 1
				sys.Inc(ZeroPage(1), &sys.X)
			},
			Check: func(sys *System) {
				assert.Equal(t, 2, sys.Bus.Memory.Read(2))
			},
		},
		{
			Name: "absolute",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x101, 1)
				sys.Inc(Absolute(0x101))
			},
			Check: func(sys *System) {
				assert.Equal(t, 2, sys.Bus.Memory.Read(0x101))
			},
		},
		{
			Name: "absolute x",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x102, 1)
				sys.X = 1
				sys.Inc(Absolute(0x101), &sys.X)
			},
			Check: func(sys *System) {
				assert.Equal(t, 2, sys.Bus.Memory.Read(0x102))
			},
		},
	}
	runCPUTest(t, tests)
}

func TestInx(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Inx()

	assert.Equal(t, 1, sys.X)
}

func TestIny(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Iny()

	assert.Equal(t, 1, sys.Y)
}

func TestJmp(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "absolute",
			Setup: func(sys *System) {
				sys.Jmp(Absolute(0x100))
			},
			Check: func(sys *System) {
				assert.Equal(t, 0x100, sys.PC)
			},
		},
		{
			Name: "indirect",
			Setup: func(sys *System) {
				sys.Bus.Memory.WriteWord(0x100, 0x200)
				sys.Jmp(Indirect(0x100))
			},
			Check: func(sys *System) {
				assert.Equal(t, 0x200, sys.PC)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestJsr(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.PC = nes.CodeBaseAddress
	sys.Jsr(Absolute(0x101))

	assert.Equal(t, cpu.InitialStack-2, sys.SP)
	assert.Equal(t, 0x101, sys.PC)
	w := sys.Pop16()
	assert.Equal(t, 0x8002, w)
}

func TestLda(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Lda(1)

	assert.Equal(t, 1, sys.A)
}

func TestLdx(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "immediate",
			Setup: func(sys *System) {
				sys.Ldx(1)
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.X)
			},
		},
		{
			Name: "zeropage y",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(2, 8)
				sys.Y = 1
				sys.Ldx(ZeroPage(1), &sys.Y)
			},
			Check: func(sys *System) {
				assert.Equal(t, 8, sys.X)
			},
		},
		{
			Name: "absolute y",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x102, 8)
				sys.Y = 1
				sys.Ldx(Absolute(0x101), &sys.Y)
			},
			Check: func(sys *System) {
				assert.Equal(t, 8, sys.X)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestLdy(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "immediate",
			Setup: func(sys *System) {
				sys.Ldy(1)
			},
			Check: func(sys *System) {
				assert.Equal(t, 1, sys.Y)
			},
		},
		{
			Name: "zeropage x",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(2, 8)
				sys.X = 1
				sys.Ldy(ZeroPage(1), &sys.X)
			},
			Check: func(sys *System) {
				assert.Equal(t, 8, sys.Y)
			},
		},
		{
			Name: "absolute x",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x102, 8)
				sys.X = 1
				sys.Ldy(Absolute(0x101), &sys.X)
			},
			Check: func(sys *System) {
				assert.Equal(t, 8, sys.Y)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestLsr(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "value 0b0000_0010 accumulator",
			Setup: func(sys *System) {
				sys.A = 0b0000_0010
				sys.Lsr()
			},
			Check: func(sys *System) {
				assert.Equal(t, 0b0000_0001, sys.A)
				assert.Equal(t, 0, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
		{
			Name: "value 0b0111_1111 accumulator",
			Setup: func(sys *System) {
				sys.A = 0b0111_1111
				sys.Lsr()
			},
			Check: func(sys *System) {
				assert.Equal(t, 0b0011_1111, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
		{
			Name: "value 0b0111_1111 absolute",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x101, 0b0111_1111)
				sys.Lsr(Absolute(0x101))
			},
			Check: func(sys *System) {
				b := sys.Bus.Memory.Read(0x101)
				assert.Equal(t, 0b0011_1111, b)
				assert.Equal(t, 0, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestNop(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Nop()
}

func TestOra(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	// TODO add test
	sys.Ora(0)
}

func TestPha(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.A = 1
	sys.Pha()

	b := sys.Bus.Memory.Read(cpu.StackBase + cpu.InitialStack)
	assert.Equal(t, sys.A, b)
	assert.Equal(t, cpu.StackBase+cpu.InitialStack-1, sys.SP)
}

func TestPhp(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Php()

	b := sys.Bus.Memory.Read(cpu.StackBase + cpu.InitialStack)
	assert.Equal(t, 0b0011_0100, b)
}

func TestPla(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.SP = 1
	sys.Bus.Memory.Write(cpu.StackBase+2, 1)
	sys.Pla()

	assert.Equal(t, 1, sys.A)
	assert.Equal(t, 2, sys.SP)
}

func TestPlp(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.SP = 1
	sys.Bus.Memory.Write(cpu.StackBase+2, 1)
	sys.Plp()

	assert.Equal(t, 0b0010_0001, sys.GetFlags())
	assert.Equal(t, 2, sys.SP)
}

func TestRol(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "value 0b0000_0010 accumulator",
			Setup: func(sys *System) {
				sys.A = 0b0000_0010
				sys.Rol()
			},
			Check: func(sys *System) {
				assert.Equal(t, 0b0000_0100, sys.A)
				assert.Equal(t, 0, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
		{
			Name: "value 0b1111_1110 accumulator C0",
			Setup: func(sys *System) {
				sys.A = 0b1111_1110
				sys.Rol()
			},
			Check: func(sys *System) {
				assert.Equal(t, 0b1111_1100, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 1, sys.Flags.N)
			},
		},
		{
			Name: "value 0b1111_1110 absolute C1",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x101, 0b1111_1110)
				sys.Flags.C = 1
				sys.Rol(Absolute(0x101))
			},
			Check: func(sys *System) {
				b := sys.Bus.Memory.Read(0x101)
				assert.Equal(t, 0b1111_1101, b)
				assert.Equal(t, 0, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 1, sys.Flags.N)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestRor(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "value 0b0000_0010 accumulator",
			Setup: func(sys *System) {
				sys.A = 0b0000_0010
				sys.Ror()
			},
			Check: func(sys *System) {
				assert.Equal(t, 0b0000_0001, sys.A)
				assert.Equal(t, 0, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
		{
			Name: "value 0b0111_1111 accumulator C0",
			Setup: func(sys *System) {
				sys.A = 0b0111_1111
				sys.Ror()
			},
			Check: func(sys *System) {
				assert.Equal(t, 0b0011_1111, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 0, sys.Flags.N)
			},
		},
		{
			Name: "value 0b0111_1111 absolute C1",
			Setup: func(sys *System) {
				sys.Bus.Memory.Write(0x101, 0b0111_1111)
				sys.Flags.C = 1
				sys.Ror(Absolute(0x101))
			},
			Check: func(sys *System) {
				b := sys.Bus.Memory.Read(0x101)
				assert.Equal(t, 0b1011_1111, b)
				assert.Equal(t, 0, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
				assert.Equal(t, 0, sys.Flags.Z)
				assert.Equal(t, 1, sys.Flags.N)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestRti(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Rti()
}

func TestRts(t *testing.T) {
	t.Parallel()
	sys := NewSystem(NewOptions())

	sys.Push16(0x100)
	sys.Rts()
	assert.Equal(t, 0x101, sys.PC)
}

func TestSbc(t *testing.T) {
	t.Parallel()
	tests := []cpuTest{
		{
			Name: "result 0xff C0",
			Setup: func(sys *System) {
				sys.A = 2
				sys.Sbc(2)
			},
			Check: func(sys *System) {
				assert.Equal(t, 0xff, sys.A)
				assert.Equal(t, 0, sys.Flags.C)
			},
		},
		{
			Name: "result 0xfe C0",
			Setup: func(sys *System) {
				sys.Sbc(1)
			},
			Check: func(sys *System) {
				assert.Equal(t, 0xfe, sys.A)
				assert.Equal(t, 0, sys.Flags.C)
			},
		},
		{
			Name: "result 0x00 C1",
			Setup: func(sys *System) {
				sys.Flags.C = 1
				sys.Sbc(0)
			},
			Check: func(sys *System) {
				assert.Equal(t, 0x00, sys.A)
				assert.Equal(t, 1, sys.Flags.C)
			},
		},
	}
	runCPUTest(t, tests)
}

func TestSec(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Sec()

	assert.Equal(t, 1, sys.Flags.C)
}

func TestSed(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Sed()

	assert.Equal(t, 1, sys.Flags.D)
}

func TestSei(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Sei()

	assert.Equal(t, 1, sys.Flags.I)
}

func TestSta(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.A = 11
	sys.Sta(0)

	b := sys.Bus.Memory.Read(0)
	assert.Equal(t, sys.A, b)

	sys.X = 0x22
	sys.Sta(Absolute(0), &sys.X)

	b = sys.Bus.Memory.Read(0x22)
	assert.Equal(t, sys.A, b)
}

func TestStx(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.X = 11
	sys.Stx(0)

	b := sys.Bus.Memory.Read(0)
	assert.Equal(t, sys.X, b)

	sys.Y = 0x22
	sys.Stx(Absolute(0), &sys.Y)

	b = sys.Bus.Memory.Read(0x22)
	assert.Equal(t, sys.X, b)
}

func TestSty(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Y = 11
	sys.Sty(0)

	b := sys.Bus.Memory.Read(0)
	assert.Equal(t, sys.Y, b)

	sys.X = 0x22
	sys.Sty(Absolute(0), &sys.X)

	b = sys.Bus.Memory.Read(0x22)
	assert.Equal(t, sys.Y, b)
}

func TestTax(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.A = 2
	sys.Tax()

	assert.Equal(t, 2, sys.X)
}

func TestTay(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.A = 2
	sys.Tay()

	assert.Equal(t, 2, sys.Y)
}

func TestTsx(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Tsx()

	assert.Equal(t, cpu.InitialStack, sys.SP)
	assert.Equal(t, cpu.InitialStack, sys.X)
}

func TestTxa(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.X = 2
	sys.Txa()

	assert.Equal(t, 2, sys.A)
}

func TestTxs(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.X = 2
	sys.Txs()

	assert.Equal(t, 2, sys.SP)
}

func TestTya(t *testing.T) {
	t.Parallel()
	sys := NewSystem(nil)

	sys.Y = 2
	sys.Tya()

	assert.Equal(t, 2, sys.A)
}
