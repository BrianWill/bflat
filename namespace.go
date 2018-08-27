package main

import (
	"errors"

)

func (ns *Namespace) GetClass(short ShortName, nsShort NSNameShort) *ClassInfo {
	if nsShort == "" {
		return ns.Classes[short]
	}
	ns = ns.Imported[nsShort]
	if ns == nil {
		return nil
	}
	return ns.Classes[short]
}

func (ns *Namespace) GetInterface(short ShortName, nsShort NSNameShort) *InterfaceInfo {
	if nsShort == "" {
		return ns.Interfaces[short]
	}
	ns = ns.Imported[nsShort]
	if ns == nil {
		return nil
	}
	return ns.Interfaces[short]
}

func (ns *Namespace) GetStruct(short ShortName, nsShort NSNameShort) *StructInfo {
	if nsShort == "" {
		return ns.Structs[short]
	}
	ns = ns.Imported[nsShort]
	if ns == nil {
		return nil
	}
	return ns.Structs[short]
}

func (ns *Namespace) GetConstructors(short ShortName, nsShort NSNameShort) []*CallableInfo {
	if nsShort == "" {
		return ns.Constructors[short]
	}
	ns = ns.Imported[nsShort]
	if ns == nil {
		return nil
	}
	return ns.Constructors[short]
}

func (ns *Namespace) GetGlobal(short ShortName, nsShort NSNameShort) *GlobalInfo {
	if nsShort == "" {
		return ns.Globals[short]
	}
	ns = ns.Imported[nsShort]
	if ns == nil {
		return nil
	}
	return ns.Globals[short]
}

func (ns *Namespace) GetFuncs(short ShortName, nsShort NSNameShort) []*CallableInfo {
	if nsShort == "" {
		return ns.Funcs[short]
	}
	ns = ns.Imported[nsShort]
	if ns == nil {
		return nil
	}
	return ns.Funcs[short]
}

func (ns *Namespace) GetMethods(short ShortName, nsShort NSNameShort) []*CallableInfo {
	if nsShort == "" {
		return ns.Methods[short]
	}
	ns = ns.Imported[nsShort]
	if ns == nil {
		return nil
	}
	return ns.Methods[short]
}

func (ns *Namespace) GetType(short ShortName, nsShort NSNameShort) Type {
	if c := ns.GetClass(short, nsShort); c != nil {
		return c
	}
	if s := ns.GetStruct(short, nsShort); s != nil {
		return s
	}
	if i := ns.GetInterface(short, nsShort); i != nil {
		return i
	}

	// return BuiltinType or ArrayType if a validd type
	if nsShort == "" {
		switch short {
		case "A":
			// todo: account for ArrayType (need to pass in type params?)
			return nil
		case "I":
			return IntType
		case "II":
			return LongType
		case "F":
			return FloatType
		case "FF":
			return DoubleType
		case "B":
			return ByteType
		case "SB":
			return SignedByteType
		}
	}
	return nil
}

func (ns *Namespace) HasName(short ShortName) {
	if ns.Methods[short] != nil {
		return true
	}
	if ns.Funcs[short] != nil {
		return true
	}
	if ns.Classes[short] != nil {
		return true
	}
	if ns.Structs[short] != nil {
		return true
	}
	if ns.Interfaces[short] != nil {
		return true
	}
	if ns.Globals[short] != nil {
		return true
	}
	return false
}



func createNamespace(topDefs *TopDefs, namespace NSNameFull, basedir string, namespaces map[NSNameFull]*Namespace) (*Namespace, error) {
	if topDefs.Namespace.Name == "" {
		return nil, errors.New("Namespace '" + string(namespace) + "' missing its namespace declaration.")
	}
	if topDefs.Namespace.Name != namespace {
		return nil, errors.New("Namespace '" + string(namespace) + "' declaration does not match expected name.")
	}
	ns := &Namespace{
		Name:         namespace,
		Imported:     map[NSNameShort]*Namespace{},
		Classes:      map[ShortName]*ClassInfo{},
		Structs:      map[ShortName]*StructInfo{},
		Interfaces:   map[ShortName]*InterfaceInfo{},
		Globals:      map[ShortName]*GlobalInfo{},
		Constructors: map[ShortName][]*CallableInfo{},
		Funcs:        map[ShortName][]*CallableInfo{},
		Methods:      map[ShortName][]*CallableInfo{},
	}

	for _, importDef := range topDefs.Imports {
		if _, ok := ns.Imports[importDef.Shortname]; ok {
			return nil, errors.New("Name collision between imported namespace short names: " + string(importDef.Shortname))
		}

		foreign, ok := namespaces[importDef.Namespace]
		if !ok {
			err := compileNamespace(importDef.Namespace, basedir, namespaces)
			if err != nil {
				return nil, err
			}
			foreign = namespaces[importDef.Namespace]
		}
		ns.Imports[importDef.Shortname] = foreign

		for name, interfaceInfo := range foreign.Interfaces {
			if interfaceInfo.Namespace == foreign {
				if ns.HasName(name) {
					return nil, errors.New("Name collision: " + string(name) + " imported from more than one namespaces.")
				}
				ns.Interfaces[name] = interfaceInfo
			}
		}

		for name, classInfo := range foreign.Classes {
			if classInfo.Namespace == foreign {
				if ns.HasName(name) {
					return nil, errors.New("Name collision: " + string(name) + " imported from more than one namespaces.")
				}
				ns.Classes[name] = classInfo
			}
		}

		for name, structInfo := range foreign.Structs {
			if structInfo.Namespace == foreign {
				if ns.HasName(name) {
					return nil, errors.New("Name collision: " + string(name) + " imported from more than one namespaces.")
				}
				ns.Structs[name] = structInfo
			}
		}

		for name, globalInfo := range foreign.Globals {
			if globalInfo.Namespace == foreign {
				if ns.HasName(name) {
					return nil, errors.New("Name collision: " + string(name) + " imported from more than one namespaces.")
				}
				ns.Globals[name] = globalInfo
			}
		}

		// we don't look for function signature conflicts between because
		// we only care about conflicts at call sites

		for name, callables := range foreign.Funcs {
			for _, callable := range callables {
				if callable.Namespace == foreign {
					ns.Funcs[name] = append(ns.Funcs[name], callable)
				}
			}
		}

		for name, callables := range foreign.Constructors {
			if callables[0].Namespace == foreign {
				if ns.HasName(name) {
					return nil, errors.New("Name collision: " + string(name) + " imported from more than one namespaces.")
				}
				ns.Constructors[name] = callables
			}
		}

		// (impossible to have method signature name conflicts)
		for name, callables := range foreign.Methods {
			for _, callable := range callables {
				if callable.Namespace == foreign {
					ns.Methods[name] = append(ns.Methods[name], callable)
				}
			}
		}
	}

	for i, interfaceDef := range topDefs.Interfaces {
		if ns.HasName(interfaceDef.Type.Name) {
			return nil, msg(interfaceDef.Line, interfaceDef.Column, "Interface name already used.")
		}

		ns.Interfaces[interfaceDef.Type.Name] = &InterfaceInfo{
			Name:      interfaceDef.Type.Name,
			Namespace: ns,
		}
	}

	for i, structDef := range topDefs.Structs {
		if ns.HasName(structDef.Type.Name) {
			return nil, msg(structDef.Line, structDef.Column, "Struct name already used.")
		}

		structInterfaces := []*InterfaceInfo{}
		for _, dt := range structDef.Interfaces {
			interfaceInfo := ns.GetInterface(dt.Name, dt.Namespace)
			if interfaceInfo == nil {
				return nil, msg(structDef.Line, structDef.Column, "Struct implements unknown interface.")
			}
			structInterfaces = append(structInterfaces, interfaceInfo)
		}

		ns.Structs[structDef.Type.Name] = &StructInfo{
			Name:       structDef.Type.Name,
			Namespace:  ns,
			Interfaces: structInterfaces,
		}
	}

	for _, globalDef := range topDefs.Globals {
		if ns.HasName(globalDef.Type.Name) {
			return nil, msg(globalDef.Line, globalDef.Column, "Global name already used.")
		}
		ns.Globals[globalDef.Name] = &GlobalInfo{
			Name:      globalDef.Name,
			Namespace: ns,
		}
	}

	for _, classDef := range topDefs.Classes {
		if ns.HasName(classDef.Type.Name) {
			return nil, msg(classDef.Line, classDef.Column, "Class name already used.")
		}

		ns.Classes[classDef.Type.Name] = &ClassInfo{
			Name:      classDef.Type.Name,
			Namespace: ns,
		}
	}

	// set up interface methods
	for _, interfaceDef := range topDefs.Interfaces {
		interfaceInfo := ns.GetInterface(interfaceDef.Type.Name, interfaceDef.Type.Namespace)


		methodSigs := map[string][][]Type{}
		for i, methodName := range interfaceDef.MethodNames {
			methodParams := interfaceDef.MethodParams[i]
			methodReturn := interfaceDef.MethodReturnTypes[i]

			returnType := ns.GetType(methodReturn.Name, methodReturn.Namespace)
			if returnType == nil {
				return nil, msg(interfaceDef.Line, interfaceDef.Column, "Method return type is of unknown type: " + methodReturn.Name + "/" + methodReturn.Namespace)
			}

			types, err := getParamTypes(methodParams)
			if err != nil {
				return nil, err
			}

			if signatureConflict(types, methodSigs[methodName]) {
				return nil, msg(interfaceDef.Line, interfaceDef.Column, "Two or more methods in an interface have the same name and parameter types, so all calls would be ambiguous: " + method.Name)
			}

			methodSigs[methodName] = append(methodSigs[methodName], types)

			callable := &CallableInfo{
				IsMethod:   true,
				Namespace:  ns,
				ParamNames: make([]string, len(types) + 1),       // in case len(ParamNames) used for looping over params       
				ParamTypes: append([]Type{interfaceInfo}, types...),
				Return: returnType,
			}

			ns.Methods[methodName] = append(ns.Methods[methodName], callable)
			interfaceInfo.Methods = append(interfaceInfo.Methods, callable)
		}
	}

	// init ClassInfo Parent, Interfaces, Fields, constructors, and methods
	for _, classDef := range topDefs.Classes {
		classInfo := ns.Classes[classDef.Type.Name] // should never be nil

		interfaces := []*InterfaceInfo
		for i, dt := range classDef.Supertypes {
			if i == 0 {
				parentClass := ns.GetClass(dt.Name, dt.Namespace)
				if parentClass != nil {
					classInfo.Parent = parentClass
					continue
				}
			}
			interfaceInfo := ns.GetInterface(dt.Name, dt.Namespace)
			if interfaceInfo == nil {
				if i == 0 {
					return nil, msg(classDef.Line, classDef.Column, "Class has unknown parent or implements unknown interface.")
				}
				return nil, msg(classDef.Line, classDef.Column, "Class implements unknown interface.")
			}
			interfaces = append(interfaces, interfaceInfo)
		}
		classInfo.Interfaces = interfaces

		classInfo.Fields = map[ShortName]FieldInfo{}
		for _, f := range classDef.Fields {
			t := ns.GetType(f.Type.Name, f.Type.Namespace)
			if t == nil {
				return nil, msg(f.Line, f.Column, "Field has unknown type.")
			}
			classInfo.Fields[f.Name] = FieldInfo{
				Name:        f.Name,
				Type:        t,
				AccessLevel: f.AccessLevel,
			}
		} 

		hasZeroArgConstructor := false
		
		constructorSigs := [][]Type{} 
		for _, constructor := range classDef.Constructors {
			if len(constructor.ParamNames) == 0 {
				hasZeroArgConstructor = true
			}

			types, err := getParamTypes(constructor.ParamTypes)
			if err != nil {
				return nil, err
			}

			if signatureConflict(types, constructorSigs) {
				return nil, msg(fn.Line, fn.Column, "Two or more constructors of the same class have the same parameter types, so all calls would be ambiguous:" + classDef.Name + "/" + classDef.Namespace)
			}

			constructorSigs = append(constructorSigs, types)

			ns.Constructors[classDef.Type.Name] = append(ns.Constructors[classDef.Type.Name],
				&CallableInfo{
					IsMethod:   false,
					Namespace:  ns,
					ParamNames: constructor.ParamNames,
					ParamTypes: types,
					Return: classInfo,
				},
			)
		}

		// add sig for default constructor if none explicitly defined
		if !hasZeroArgConstructor {
			ns.Constructors[classDef.Type.Name] = append(ns.Constructors[classDef.Type.Name],
				&CallableInfo{
					IsMethod:   false,
					Namespace:  ns,
					ParamNames: nil,
					ParamTypes: nil,
					Return: classInfo,
				},
			)
		}

		classInfo.Methods = map[ShortName][]*CallableInfo{}
		methodSigs := map[string][][]Type{} 
		for _, method := range classDef.Methods {
			returnType := ns.GetType(method.Return.Name, method.Return.Namespace)
			if returnType == nil {
				return nil, msg(method.Line, method.Column, "Method return type is of unknown type: " + method.Return.Name + "/" + method.Return.Namespace)
			}

			types, err := getParamTypes(method.ParamTypes)
			if err != nil {
				return nil, err
			}

			if signatureConflict(types, methodSigs[method.Name]) {
				return nil, msg(method.Line, method.Column, "Two or more methods of the same class have the same name and parameter types, so all calls would be ambiguous: " + method.Name)
			}

			methodSigs[method.Name] = append(methodSigs[method.Name], types)

			callable := &CallableInfo{
				IsMethod:   true,
				Namespace:  ns,
				ParamNames: append([]string{thisWord}, method.ParamNames...),
				ParamTypes: append([]Type{classInfo}, types...),
				Return: returnType,
			}

			ns.Methods[method.Name] = append(ns.Methods[method.Name], callable)
			classInfo.Methods[method.Name] = append(classInfo.Methods[method.Name], callable)
		}

		// check that the class actually implements the interfaces
		for _, interfaceInfo := range classInfo.Interfaces {
			// todo: account for methods of ancestor interfaces

			for name, im := range interfaceInfo.Methods {
				match := false
			Loop:
				for _, cm := range classInfo.Methods[name] {
					if cm.Return != im.Return {
						continue
					}
					if len(im.ParamTypes) != len(cm.ParamTypes) {
						continue
					}
					for i := 1; i < len(cm.ParamTypes); i++ {
						if cm.ParamTypes[i] != im.ParamTypes[i] {
							continue Loop
						}
					}
					match = true
					break
				}

				if !match {
					return nil, msg(classDef.Line, classDef.Column, "Class "+string(classInfo.Name)+" does not implement method "+
						string(name)+" of interface "+string(interfaceInfo.Name)+"/"+string(interfaceInfo.Namespace)+".")
				}
			}
		}
	}

	funcSigs := map[string][][]Type{} 
	for _, fn := range topDefs.Funcs {

		returnType := ns.GetType(fn.Return.Name, fn.Return.Namespace)
		if returnType == nil {
			return nil, msg(fn.Line, fn.Column, "Function return type is of unknown type:" + fn.Return.Name + "/" + fn.Return.Namespace)
		}

		types, err := getParamTypes(fn.ParamTypes)
		if err != nil {
			return nil, err
		}

		if signatureConflict(types, funcSigs[fn.Name]) {
			return nil, msg(fn.Line, fn.Column, "Two or more functions with same name in this namespace have the same parameter types, so all calls would be ambiguous: " + fn.Name + "/" + fn.Namespace)
		}

		funcSigs[fn.Name] = append(funcSigs[fn.Name], types)

		ns.Funcs[fn.Name] = append(ns.Funcs[fn.Name],
			&CallableInfo{
				IsMethod:    false,
				Namespace:   ns,
				ParamNames:  fn.ParamNames,
				ParamTypes:  types,
				Return:  returnType,
				StaticClass: fn.StaticClass,
			},
		)
	}


	// init global Type fields
	for _, globalDef := range topDefs.Globals {
		globalInfo := ns.Globals[globalDef.Name]
		t := ns.GetType(globalDef.Type.Name, globalDef.Type.Namespace)
		if t == nil {

		}
		globalInfo.Type = t
	}


	// init StructInfo Interfaces, Fields, constructors, and methods
	for _, structDef := range topDefs.Structs {
		structInfo := ns.Structs[structDef.Type.Name] // should never be nil

		interfaces := []*InterfaceInfo
		for i, dt := range structDef.Interfaces {
			interfaceInfo := ns.GetInterface(dt.Name, dt.Namespace)
			if interfaceInfo == nil {
				return nil, msg(structDef.Line, structDef.Column, "Struct implements unknown interface.")
			}
			interfaces = append(interfaces, interfaceInfo)
		}
		structInfo.Interfaces = interfaces

		structInfo.Fields = map[ShortName]FieldInfo{}
		for _, f := range structDef.Fields {
			t := ns.GetType(f.Type.Name, f.Type.Namespace)
			if t == nil {
				return nil, msg(f.Line, f.Column, "Field has unknown type.")
			}
			structInfo.Fields[f.Name] = FieldInfo{
				Name:        f.Name,
				Type:        t,
				AccessLevel: f.AccessLevel,
			}
		} 

		hasZeroArgConstructor := false
		
		constructorSigs := [][]Type{} 
		for _, constructor := range structDef.Constructors {
			if len(constructor.ParamNames) == 0 {
				hasZeroArgConstructor = true
			}

			types, err := getParamTypes(constructor.ParamTypes)
			if err != nil {
				return nil, err
			}

			if signatureConflict(types, constructorSigs) {
				return nil, msg(constructor.Line, constructor.Column, "Two or more constructors of the same class have the same parameter types, so all calls would be ambiguous: " + structDef.Name + "/" + structDef.Namespace)
			}

			constructorSigs = append(constructorSigs, types)

			ns.Constructors[structDef.Type.Name] = append(ns.Constructors[structDef.Type.Name],
				&CallableInfo{
					IsMethod:   false,
					Namespace:  ns,
					ParamNames: constructor.ParamNames,
					ParamTypes: types,
					Return: structInfo,
				},
			)
		}

		// add sig for default constructor if none explicitly defined
		if !hasZeroArgConstructor {
			ns.Constructors[structDef.Type.Name] = append(ns.Constructors[structDef.Type.Name],
				&CallableInfo{
					IsMethod:   false,
					Namespace:  ns,
					ParamNames: nil,
					ParamTypes: nil,
					Return: structInfo,
				},
			)
		}

		structInfo.Methods = map[ShortName][]*CallableInfo{}
		methodSigs := map[string][][]Type{} 
		for _, method := range structDef.Methods {
			returnType := ns.GetType(method.Return.Name, method.Return.Namespace)
			if returnType == nil {
				return nil, msg(method.Line, method.Column, "Method return type is of unknown type: " + method.Return.Name + "/" + method.Return.Namespace)
			}

			types, err := getParamTypes(method.ParamTypes)
			if err != nil {
				return nil, err
			}

			if signatureConflict(types, methodSigs[method.Name]) {
				return nil, msg(method.Line, method.Column, "Two or more methods of the same struct have the same name and parameter types, so all calls would be ambiguous: " + method.Name)
			}

			methodSigs[method.Name] = append(methodSigs[method.Name], types)

			callable := &CallableInfo{
				IsMethod:   true,
				Namespace:  ns,
				ParamNames: append([]string{thisWord}, method.ParamNames...),
				ParamTypes: append([]Type{structInfo}, types...),
				Return: returnType,
			}

			ns.Methods[method.Name] = append(ns.Methods[method.Name], callable)
			structInfo.Methods[method.Name] = append(structInfo.Methods[method.Name], callable)
		}

		// check that the class actually implements the interfaces
		for _, interfaceInfo := range structInfo.Interfaces {
			// todo: account for methods of ancestor interfaces

			for name, im := range interfaceInfo.Methods {
				match := false
			Loop:
				for _, sm := range structInfo.Methods[name] {
					if sm.Return != im.Return {
						continue
					}
					if len(im.ParamTypes) != len(sm.ParamTypes) {
						continue
					}
					for i := 1; i < len(sm.ParamTypes); i++ {
						if sm.ParamTypes[i] != im.ParamTypes[i] {
							continue Loop
						}
					}
					match = true
					break
				}

				if !match {
					return nil, msg(structDef.Line, structDef.Column, "Struct "+string(structInfo.Name)+" does not implement method "+
						string(name)+" of interface "+string(interfaceInfo.Name)+"/"+string(interfaceInfo.Namespace)+".")
				}
			}
		}
	}

	return ns, nil
}



func signatureConflict(sigTypes []Type, otherSigTypes [][]Type) bool {
	for _, otherTypes := range otherSigTypes {
		if len(otherTypes) != len(sigTypes) {
			continue
		}
		same := true
		for i := 0; i < len(otherTypes); i++ {
			if sigTypes[i] != otherTypes[i] {
				same = false
				break
			}
		}
		if same {
			return true
		}
	}
	return false
}


func getParamTypes(typeAtoms []TypeAtom) ([]Type, error) {
	types := make([]Type, len(typeAtoms))
	for i, ta := range typeAtoms {
		t := ns.GetType(ta.Name, ta.Namespace)
		if t == nil {
			return nil, msg(ta.Line, ta.Column, "Parameter has unknown type:" + ta.Name + "/" + ta.Namespace)
		}
		types[i] = t
	}
	return types, nil
}




// returns true if field exists
// todo: account for access level
func GetFieldType(field string, t Type) (Type, bool) {
	switch t := t.(type) {
	case *ClassInfo:
		// must search ancestors as well as the class itself
		for {
			if fieldInfo, ok := t.Fields[field]; ok {
				return fieldInfo.Type, true
			}
			if t.Parent == nil {
				break
			}
			t = t.Parent
		}
		return nil, false
	case *StructInfo:
		if fieldInfo, ok := t.Fields[field]; ok {
			return fieldInfo.Type, true
		} else {
			return nil, false
		}
	case *InterfaceInfo:
		// todo (interfaces can have properties)
		return nil, false
	case ArrayType:
		return nil, false
	case BuiltinType:
		if t == StrType {
			if field == StrLengthWord {
				return IntType, true
			}
		}
	}
}





// return false if other is not an interface
// return true if t implements interface other
func IsImplementor(t Type, other Type) bool {
	interfaceInfo, ok := other.(*InterfaceInfo)
	if !ok {
		return false
	}

	switch t := t.(type) {
	case *StructInfo:
		for _, ii := range t.Interfaces {
			if IsDescendent(ii, interfaceInfo) {
				return true
			}
		}
	case *ClassInfo:
		for _, ii := range t.Interfaces {
			if IsDescendent(ii, interfaceInfo) {
				return true
			}
		}
		for _, parent := range t.Parents {
			if IsImplementor(parent, interfaceInfo) {
				return true
			}
		}
	case *InterfaceInfo:
		return IsDescendent(t, other)
	case ArrayType:
		return false
	case BuiltinType:
		return false
	}
	return false
}

// true if both are classes or interfaces and t is descendent of other (or if t == other)
func IsDescendent(t Type, other Type) bool {
	if t == other {
		return true
	}

	switch t := t.(type) {
	case *ClassInfo:
		if other, ok := other.(*ClassInfo); ok {
			for _, p := range t.Parents {
				if IsDescendent(p, other) {
					return true
				}
			}
		}
	case *StructInfo:
		return false        // structs cannot descend from other structs or classes
	case *InterfaceInfo:
		if other, ok := other.(*InterfaceInfo); ok {
			for _, p := range t.Parents {
				if IsDescendent(p, other) {
					return true
				}
			}
		}
	case ArrayType:
		return false
	case BuiltinType:
		if other, ok := other.(BuiltinType); ok {
			switch other.Name {
			case "I":
				switch t.Name {
				case "I", "B", "SB":
					return true
				}
			case "II":
				switch t.Name {
				case "II", "I", "B", "SB":
					return true
				}
			case "F":
				switch t.Name {
				case "F", "B", "SB":
					return true
				}
			case "FF":
				switch t.Name {
				case "FF", "F", "I", "B", "SB":
					return true
				}
			case "B":
				return t.Name == "B"
			case "SB":
				return t.Name == "SB"
			case "Str":
				return t.Name == "Str"
			}
		}
		return false
	}
	return false
}



func IsSubType(t Type, other Type) bool {
	if other == AnyType {
		return true
	}
	if t == other {
		return true
	}
	if IsDescendent(t, other) {
		return true
	}
	if IsImplementor(t, other) {
		return true
	}
	switch t := t.(type) {
	case ArrayType:
		if other, ok := other.(ArrayType); ok {
			return IsSubType(t.BaseType, other.BaseType)
		}
	}
	return false
}



// return base type and dimension
func GetArrayType(arr ArrayType) (base Type, dimensions int) {
	dimensions = 1
	for next, ok := arr.BaseType.(ArrayType); ok; {	
		dimensions++
		arr = next
	}
	return arr.BaseType, dimensions
}

func IsNumber(t Type) bool {
	switch t := t.(type) {
	case BuiltinType:
		switch t.Name {
		case "I", "II", "F", "FF", "B", "SB":
			return true
		}
	}
	return false
}

func IsInteger(t Type) bool {
	switch t := t.(type) {
	case BuiltinType:
		switch t.Name {
		case "I", "II", "B", "SB":
			return true
		}
	}
	return false
}