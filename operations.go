package main

func isNumber(dt DataType) bool {
	return dt.Name == "I" || dt.Name == "F" || dt.Name == "Byte" || dt.Name == "II" || dt.Name == "FF"
}

func isInteger(dt DataType) bool {
	return dt.Name == "I" || dt.Name == "Byte" || dt.Name == "II" || dt.Name == "SByte"
}

func compileOperation(op CallForm, ns *Namespace, expectedType DataType,
	locals map[string]DataType) (string, DataType, error) {
	returnType := expectedType
	expectedArgType := expectedType
	multiOperand := true
	switch op.Name {
	case "add", "sub", "mul", "div":
		if !isNumber(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
	case "lt", "lte", "gt", "gte":
		if !isNumber(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
		expectedArgType = DoubleType // todo: actually need type which is supertype of all numbers (Long is not subtype of Double)
	case "inc", "dec":
		multiOperand = false
		if !isInteger(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
	case "mod", "band", "bor", "bxor":
		if !isInteger(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
	case "bnot":
		if !isInteger(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
		multiOperand = false
	case "eq", "neq":
		expectedArgType = DataType{}
		returnType = BoolType
	case "not":
		multiOperand = false
		expectedArgType = BoolType
	case "or", "and":
		expectedArgType = BoolType
	case "cat":
		expectedArgType = StrType
	default:
		return "", DataType{}, msg(op.Line, op.Column, "Unknown operation or call to unknown function or method.")
	}
	if multiOperand {
		if len(op.Args) < 2 {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation requires at least two operands")
		}
	} else {
		if len(op.Args) != 1 {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation requires one operand")
		}
	}
	operandCode := make([]string, len(op.Args))
	operandTypes := make([]DataType, len(op.Args))
	for i, expr := range op.Args {
		var err error
		operandCode[i], operandTypes[i], err = compileExpression(expr, ns, expectedArgType, locals)
		if err != nil {
			return "", DataType{}, err
		}
	}
	code := "("
	switch op.Name {
	case "add", "sub", "mul", "div", "mod", "and", "or", "band", "bor", "bxor", "cat":
		operatorSymbol := OperatorSymbols[op.Name]
		for i := range op.Args {
			code += operandCode[i]
			if i < len(op.Args)-1 {
				code += operatorSymbol
			}
		}
	case "inc":
		code += operandCode[0] + " + 1"
	case "dec":
		code += operandCode[0] + " - 1"
	case "eq", "neq":
		operatorSymbol := OperatorSymbols[op.Name]
		for i := 0; i < len(op.Args)-1; i++ {
			if !isType(operandTypes[i+1], operandTypes[0], ns, true) {
				return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation has mismatched operand types")
			}
			if i > 0 {
				code += " && "
			}
			code += operandCode[i] + operatorSymbol + operandCode[i+1]
		}
	case "not":
		code += "!" + operandCode[0]
	case "lt", "gt", "gte", "lte":
		operatorSymbol := OperatorSymbols[op.Name]
		for i := 0; i < len(op.Args)-1; i++ {
			if i > 0 {
				code += " && "
			}
			// todo: should use variables to store the compiled expressions because otherwise we're potentially repeating complex expressions
			code += operandCode[i] + operatorSymbol + operandCode[i+1]
		}
	case "bnot":
		code += "^" + operandCode[1]
	}
	code += ")"
	return code, returnType, nil

}

func compileCallForm(op CallForm, ns *Namespace, expectedType DataType,
	locals map[string]DataType) (string, DataType, error) {
	fullName := fullName(op.Name, op.Namespace, ns)
	if fullName == "" {
		return compileOperation(op, ns, expectedType, locals)
	}

	argCode := make([]string, len(op.Args))
	argTypes := make([]DataType, len(op.Args))
	for i, expr := range op.Args {
		var err error
		argCode[i], argTypes[i], err = compileExpression(expr, ns, DataType{}, locals)
		if err != nil {
			return "", DataType{}, err
		}
	}
	code := ""
	var returnType DataType
	sigs := append(ns.Funcs[fullName], ns.Methods[fullName]...)
	if len(sigs) > 0 {
		matching := []int{}
		// find sigs which match args
	Loop:
		for i, sig := range sigs {
			if len(argTypes) == len(sig.ParamTypes) {
				for j, paramType := range sig.ParamTypes {
					if !isType(argTypes[j], paramType, ns, false) {
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
				code += argCode[0] + "."
			} else {
				if op.Namespace == "" {
					code += ns.Name + "." + FuncsClass + "."
				} else {
					code += op.Namespace + "." + FuncsClass + "."
				}
			}
			code += op.Name + "("
			for i, arg := range argCode {
				if isMethod && i == 0 {
					continue
				}
				if i == len(argCode)-1 {
					code += arg + ")"
				} else {
					code += arg + ","
				}
			}
			returnType = sig.ReturnType
		}
	}
	return code, returnType, nil
}
