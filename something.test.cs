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

public class Monkey : Something.Test.Bar, Something.Test.Eater {
	public string evan_;
	public string evan {
		get {return evan_;}
		set {this.evan_ = value;}
	}

	public void david() {
		string s = Something.Test.Bar.rubber;
		this.zelda = (float) 6.0;
		this.lisa();
		Something.Test.Bar b = new Something.Test.Bar();
		this.evan = s;
		s = this.evan;
	}
}

public class Bar {
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

public interface Eater {
	void david(Something.Test.Eater);
	string evan {
		get;
		set;
	}
}

}