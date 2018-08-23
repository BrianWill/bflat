namespace example {

public class _Globals {}

public class _Funcs {
	public static void main() {
		int _i = 3;
		_i = (5 + 3);
		otherspace._Funcs.tracy();
		_i = example._Funcs.kevin(2);
	}
	public static int kevin(_a int) {
		return (_a + 4);
	}
}

public Foo {
	public int alice = 24;

	public float bar(_a int, _c int) {
		Foo _b = new example.Foo();
		this.alice = 9;
		int _ack = this.alice;
		return (float) 3.0;
	}
}

public Monkey : Bar, Eater {
	public void david() {
		int _i = 3;
		_i = 5;
		this.zelda = (float) 6.0;
		this.lisa();
		this.david();
	}
}

public Bar : Foo {
	public float zelda = (float) 35.0;

	public Bar() {
		int _i = 3;
		_i = 5;
	}
	public Bar(_a string) {
		int _i = 3;
		_i = 5;
	}
	public void lisa() {
		int _i = 3;
		_i = 5;
		Eater _test;
		_test = new example.Monkey();
		_test.david();
	}
}

public Eater {
	public void david();
}

}