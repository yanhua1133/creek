package commonast

// Role 表示子节点在父节点中的语义职责。
type Role uint8

const (
	// RoleNone 表示没有可识别的语义职责。
	RoleNone Role = iota
	// RoleName 表示声明或引用名称。
	RoleName
	// RoleBody 表示主体语句或主体声明集合。
	RoleBody
	// RoleCondition 表示条件表达式。
	RoleCondition
	// RoleInit 表示初始化部分。
	RoleInit
	// RoleUpdate 表示循环更新部分。
	RoleUpdate
	// RoleElse 表示条件不成立时的分支。
	RoleElse
	// RoleCallee 表示调用目标。
	RoleCallee
	// RoleArgument 表示调用实参。
	RoleArgument
	// RoleReceiver 表示方法接收者。
	RoleReceiver
	// RoleReturnType 表示函数返回类型。
	RoleReturnType
	// RoleType 表示声明或表达式关联的类型。
	RoleType
	// RoleValue 表示赋值、返回或初始化的值。
	RoleValue
	// RoleLeft 表示二元结构的左侧。
	RoleLeft
	// RoleRight 表示二元结构的右侧。
	RoleRight
	// RoleObject 表示成员访问或下标访问的对象。
	RoleObject
	// RoleMember 表示被访问的成员名称。
	RoleMember
	// RoleParameter 表示函数形式参数。
	RoleParameter
	// RoleTarget 表示循环或赋值目标。
	RoleTarget
	// RoleIterable 表示被遍历对象。
	RoleIterable
)

// String 返回语义角色的稳定名称。
func (r Role) String() string {
	switch r {
	case RoleName:
		return "name"
	case RoleBody:
		return "body"
	case RoleCondition:
		return "condition"
	case RoleInit:
		return "init"
	case RoleUpdate:
		return "update"
	case RoleElse:
		return "else"
	case RoleCallee:
		return "callee"
	case RoleArgument:
		return "argument"
	case RoleReceiver:
		return "receiver"
	case RoleReturnType:
		return "return_type"
	case RoleType:
		return "type"
	case RoleValue:
		return "value"
	case RoleLeft:
		return "left"
	case RoleRight:
		return "right"
	case RoleObject:
		return "object"
	case RoleMember:
		return "member"
	case RoleParameter:
		return "parameter"
	case RoleTarget:
		return "target"
	case RoleIterable:
		return "iterable"
	default:
		return "none"
	}
}
