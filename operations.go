package main

func isNumber(dt DataType) bool {
	return dt.Name == "I" || dt.Name == "F" || dt.Name == "Byte" || dt.Name == "II" || dt.Name == "FF"
}

func isInteger(dt DataType) bool {
	return dt.Name == "I" || dt.Name == "Byte" || dt.Name == "II" || dt.Name == "SByte"
}

func compileCallForm(op CallForm, namespace *Namespace, locals map[string]DataType) (string, DataType, error) {
	operandCode := make([]string, len(op.Args))
	operandTypes := make([]DataType, len(op.Args))
	for i, expr := range op.Args {
		var err error
		operandCode[i], operandTypes[i], err = compileExpression(expr, namespace, DataType{}, locals)
		if err != nil {
			return "", DataType{}, err
		}
	}
	code := "("
	var returnType DataType
	sigs, names := getSignatures(op.Name, op.Namespace, namespace)
	if len(sigs) > 0 {
		matching := []int{}
		// find sigs which match args
	Loop:
		for i, sig := range sigs {
			if len(operandTypes) == len(sig.ParamTypes) {
				for j, paramType := range sig.ParamTypes {
					if !isType(operandTypes[j], paramType, namespace, false) {
						continue Loop
					}
				}
				matching = append(matching, i)
			}
		}
		if len(matching) > 1 {
			return "", DataType{}, msg(op.Line, op.Column, "Call is ambiguous (multiple matching methods or functions).")
		} else if len(matching) == 1 {
			sig := sigs[matching[0]]
			isMethod := sig.IsMethod
			if isMethod {
				code += operandCode[0] + "."
			}
			code += names[matching[0]] + "("
			for i, arg := range operandCode {
				if isMethod && i == 0 {
					continue
				}
				if i == len(operandCode)-1 {
					code += arg + ")"
				} else {
					code += arg + ","
				}
			}
			returnType = sig.ReturnType
		}
	} else {
		if op.Namespace != "" {
			return "", DataType{}, msg(op.Line, op.Column, "Invalid method or function call.")
		}
		switch op.Name {
		case "add":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "'add' operations requires at least two operands.")
			}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "'add' operation has non-number operand")
			}
			for i := range op.Args {
				if !isType(operandTypes[i], t, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "'add' operation has operand whose type differs from the others")
				}
				code += operandCode[i]
				if i < len(op.Args)-1 {
					code += " + "
				}
			}
			returnType = t
		case "sub":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "'sub' operation requires at least two operands")
			}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "'sub' operation has non-number operand")
			}
			for i := range op.Args {
				if !isType(operandTypes[i], t, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "'sub' operation has operand whose type differs from the others")
				}
				code += operandCode[i]
				if i < len(op.Args)-1 {
					code += " - "
				}
			}
			returnType = t
		case "mul":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "mul operation requires at least two operands")
			}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "mul operation has non-number operand")
			}
			for i := range op.Args {
				if !isType(operandTypes[i], t, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "mul operation has non-number operand")
				}
				code += operandCode[i]
				if i < len(op.Args)-1 {
					code += " * "
				}
			}
			returnType = t
		case "div":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "div operation requires at least two operands")
			}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "div operation has non-number operand")
			}
			for i := range op.Args {
				if !isType(operandTypes[i], t, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "div operation has non-number operand")
				}
				code += operandCode[i]
				if i < len(op.Args)-1 {
					code += " / "
				}
			}
			returnType = t
		case "inc":
			if len(op.Args) != 1 {
				return "", DataType{}, msg(op.Line, op.Column, "inc operation requires one operand.")
			}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "inc operation has non-number operand")
			}
			code += operandCode[0] + " + 1"
			returnType = t
		case "dec":
			if len(op.Args) != 1 {
				return "", DataType{}, msg(op.Line, op.Column, "dec operation requires one operand.")
			}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "dec operation has non-number operand")
			}
			code += operandCode[0] + " - 1"
			returnType = t
		case "mod":
			if len(op.Args) != 2 {
				return "", DataType{}, msg(op.Line, op.Column, "mod operation requires two operands")
			}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "mod operation's first operand must be a number")
			}
			if !isType(operandTypes[1], t, namespace, true) {
				return "", DataType{}, msg(op.Line, op.Column, "mod operation's second operand must be same type as first")
			}
			for i := range op.Args {
				code += operandCode[i]
				if i < len(op.Args)-1 {
					code += " % "
				}
			}
			returnType = t
		case "eq":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "eq operation requires at least two operands")
			}
			returnType = DataType{Name: "Bool"}
			for i := 0; i < len(op.Args)-1; i++ {
				if !isType(operandTypes[i], operandTypes[0], namespace, true) ||
					!isType(operandTypes[i+1], operandTypes[0], namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "eq operation has mismatched operand types")
				}
				if i > 0 {
					code += " && "
				}
				code += operandCode[i] + " == " + operandCode[i+1]
			}
		case "neq":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "neq operation requires at least two operands")
			}
			returnType = DataType{Name: "Bool"}
			for i := 0; i < len(op.Args)-1; i++ {
				if !isType(operandTypes[i], operandTypes[0], namespace, true) ||
					!isType(operandTypes[i+1], operandTypes[0], namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "neq operation has mismatched operand types")
				}
				if i > 0 {
					code += " && "
				}
				code += operandCode[i] + " != " + operandCode[i+1]
			}
		case "not":
			if len(op.Args) != 1 {
				return "", DataType{}, msg(op.Line, op.Column, "not operation requires one operand")
			}
			returnType = DataType{Name: "Bool"}
			if !isType(operandTypes[0], returnType, namespace, true) {
				return "", DataType{}, msg(op.Line, op.Column, "not operation has a non-bool operand")
			}
			code += "!" + operandCode[0]
		case "lt":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "lt operation requires at least two operands")
			}
			returnType = DataType{Name: "Bool"}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "lt operation has non-number operand")
			}
			for i := 0; i < len(op.Args)-1; i++ {
				if !isType(operandTypes[i], t, namespace, true) ||
					!isType(operandTypes[i+1], t, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "lt operation has non-number operand")
				}
				if i > 0 {
					code += " && "
				}
				code += operandCode[i] + " < " + operandCode[i+1]
			}
		case "gt":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "gt operation requires at least two operands")
			}
			returnType = DataType{Name: "Bool"}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "lt operation has non-number operand")
			}
			for i := 0; i < len(op.Args)-1; i++ {
				if !isType(operandTypes[i], t, namespace, true) ||
					!isType(operandTypes[i+1], t, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "gt operation has non-number operand")
				}
				if i > 0 {
					code += " && "
				}
				code += operandCode[i] + " > " + operandCode[i+1]
			}
		case "lte":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "lte operation requires at least two operands")
			}
			returnType = DataType{Name: "Bool"}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "lt operation has non-number operand")
			}
			for i := 0; i < len(op.Args)-1; i++ {
				if !isType(operandTypes[i], t, namespace, true) ||
					!isType(operandTypes[i+1], t, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "lte operation has non-number operand")
				}
				if i > 0 {
					code += " && "
				}
				code += operandCode[i] + " <= " + operandCode[i+1]

			}
		case "gte":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "gte operation requires at least two operands")
			}
			returnType = DataType{Name: "Bool"}
			t := operandTypes[0]
			if !isNumber(t) {
				return "", DataType{}, msg(op.Line, op.Column, "lt operation has non-number operand")
			}
			for i := 0; i < len(op.Args)-1; i++ {
				if !isType(operandTypes[i], t, namespace, true) ||
					!isType(operandTypes[i+1], t, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "gte operation has non-number operand")
				}
				if i > 0 {
					code += " && "
				}
				code += operandCode[i] + " >= " + operandCode[i+1]
			}
		case "or":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "or operation requires at least two operands")
			}
			returnType = DataType{Name: "Bool"}
			for i := range op.Args {
				if !isType(operandTypes[i], returnType, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "or operation has non-boolean operand")
				}
				code += operandCode[i]
				if i < len(op.Args)-1 {
					code += " || "
				}
			}
		case "and":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "and operation requires at least two operands")
			}
			returnType = DataType{Name: "Bool"}
			for i := range op.Args {
				if !isType(operandTypes[i], returnType, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "and operation has non-boolean operand")
				}
				code += operandCode[i]
				if i < len(op.Args)-1 {
					code += " && "
				}
			}
		case "band":
			if len(op.Args) != 2 {
				return "", DataType{}, msg(op.Line, op.Column, "'band' operation requires two operands")
			}
			if !isNumber(operandTypes[0]) || !isNumber(operandTypes[1]) {
				return "", DataType{}, msg(op.Line, op.Column, "'band' operation requires two number operands")
			}
			code += operandCode[0] + " & " + operandCode[1]
		case "bor":
			if len(op.Args) != 2 {
				return "", DataType{}, msg(op.Line, op.Column, "'bor' operation requires two operands")
			}
			if !isNumber(operandTypes[0]) || !isNumber(operandTypes[1]) {
				return "", DataType{}, msg(op.Line, op.Column, "'bor' operation requires two number operands")
			}
			code += operandCode[0] + " | " + operandCode[1]
		case "bxor":
			if len(op.Args) != 2 {
				return "", DataType{}, msg(op.Line, op.Column, "'bxor' operation requires two operands")
			}
			if !isNumber(operandTypes[0]) || !isNumber(operandTypes[1]) {
				return "", DataType{}, msg(op.Line, op.Column, "'bxor' operation requires two number operands")
			}
			code += operandCode[0] + " ^ " + operandCode[1]
		case "bnot":
			if len(op.Args) != 1 {
				return "", DataType{}, msg(op.Line, op.Column, "'bnot' operation requires one operand")
			}
			if !isNumber(operandTypes[0]) {
				return "", DataType{}, msg(op.Line, op.Column, "'bnot' operation requires one number operand")
			}
			code += "^" + operandCode[1]
		case "cat":
			if len(op.Args) < 2 {
				return "", DataType{}, msg(op.Line, op.Column, "concat operation requires at least two operands")
			}
			returnType = DataType{Name: "Str"}
			for i := range op.Args {
				if !isType(operandTypes[i], returnType, namespace, true) {
					return "", DataType{}, msg(op.Line, op.Column, "concat operation has non-string operand")
				}
				code += operandCode[i]
				if i < len(op.Args)-1 {
					code += " + "
				}
			}
		}
	}

	code += ")"
	return code, returnType, nil
}
