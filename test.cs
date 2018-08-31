namespace Something.Test {

public class _Globals {
	public string foo = "234";
}

public class _Funcs {
	public static void main() {
		Something.Test.Bar b;
		b = new Something.Test.Bar();
		int[] x;
		x = new int[6];
	}
}

public Monkey : Something.Test.Bar, Something.Test.Eater {
	public void david() {
		string s = Something.Test.Bar.rubber;
		this.zelda = (float) 6.0;
		this.lisa();
		Something.Test.Bar b = new Something.Test.Bar();
	}
}

public Bar {
	public static string rubber;
	public float zelda = (float) 35.0;

	public Bar() {
		int i = 3;
		i = 5;
	}
	public Bar(a string) {
		string i;
		i = "jsidfj";
	}
	public void lisa() {
		byte i = (byte) 3;
		i = (byte) 5;
	}
}

public Eater {
	public void david(Something.Test.Eater);
}

}