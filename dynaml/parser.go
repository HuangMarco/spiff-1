package dynaml

import (
	"container/list"
	"strconv"
	"strings"

	"github.com/mandelsoft/spiff/debug"
)

type helperNode struct{}

func (e helperNode) Evaluate(binding Binding, locally bool) (interface{}, EvaluationInfo, bool) {
	panic("not intended to be evaluated")
}

type expressionListHelper struct {
	helperNode
	list []Expression
}

type nameListHelper struct {
	helperNode
	list []string
}

type nameHelper struct {
	helperNode
	name string
}

type operationHelper struct {
	helperNode
	op string
}

func Parse(source string, path []string, stubPath []string) (Expression, error) {
	grammar := &DynamlGrammar{Buffer: source}
	grammar.Init()

	err := grammar.Parse()
	if err != nil {
		return nil, err
	}

	return buildExpression(grammar, path, stubPath), nil
}

func buildExpression(grammar *DynamlGrammar, path []string, stubPath []string) Expression {
	tokens := &tokenStack{}

	// flags for parsing merge options in merge expression
	// this expression is NOT recursive, therefore single flag variables are sufficient
	replace := false
	required := false
	keyName := ""

	for token := range grammar.Tokens() {
		contents := grammar.Buffer[token.begin:token.end]

		switch token.pegRule {
		case ruleDynaml:
			return tokens.Pop()

		case ruleMarker:
			tokens.Push(newMarkerExpr(contents))
		case ruleSubsequentMarker:
			tokens.Pop()
			tokens.Push(tokens.Pop().(MarkerExpr).add(contents))
		case ruleMarkerExpression:
			tokens.Push(MarkerExpressionExpr{contents, tokens.Pop().(Expression)})
		case ruleMarkedExpression:
			rhs := tokens.Pop()
			if _, ok := rhs.(MarkerExpr); !ok {
				rhs = tokens.Pop().(MarkerExpr).setExpression(rhs)
			}
			tokens.Push(rhs)
		case rulePrefer:
			tokens.Push(PreferExpr{tokens.Pop()})
		case ruleAuto:
			tokens.Push(AutoExpr{path})
		case ruleMerge:
			replace = false
			required = false
			keyName = ""
		case ruleSimpleMerge:
			debug.Debug("*** rule simple merge\n")
			redirect := !equals(path, stubPath)
			tokens.Push(MergeExpr{stubPath, redirect, replace, replace || required || redirect, keyName})
		case ruleRefMerge:
			debug.Debug("*** rule ref merge\n")
			rhs := tokens.Pop()
			tokens.Push(MergeExpr{rhs.(ReferenceExpr).Path, true, replace, true, keyName})
		case ruleReplace:
			replace = true
		case ruleRequired:
			required = true
		case ruleOn:
			keyName = tokens.Pop().(nameHelper).name
		case ruleFollowUpRef:
		case ruleReference:
			tokens.Push(ReferenceExpr{strings.Split(contents, ".")})

		case ruleChained:
		case ruleChainedQualifiedExpression:
		case ruleChainedRef:
			ref := ReferenceExpr{strings.Split(contents, ".")}
			expr := tokens.Pop()
			tokens.Push(QualifiedExpr{expr, ref})
		case ruleChainedDynRef:
			ref := tokens.Pop()
			expr := tokens.Pop()
			tokens.Push(DynamicExpr{expr, ref.(Expression)})
		case ruleSlice:
			slice := tokens.Pop()
			expr := tokens.Pop()
			tokens.Push(SliceExpr{expr, slice.(RangeExpr)})

		case ruleChainedCall:
			args := tokens.PopExpressionList()
			f := tokens.Pop()
			tokens.Push(CallExpr{
				Function:  f,
				Arguments: args,
			})

		case ruleAction0:
		case ruleProjectionValue:
			value := &ProjectionValue{}
			tokens.Push(ProjectionValueExpr{value})
			tokens.Push(ProjectionValueExpr{value})

		case ruleProjection:
			qual := tokens.Pop()
			value := tokens.Pop()
			expr := tokens.Pop()
			tokens.Push(ProjectionExpr{expr, value.(ProjectionValueExpr).Value, qual})

		case ruleInteger:
			val, err := strconv.ParseInt(contents, 10, 64)
			if err != nil {
				panic(err)
			}

			tokens.Push(IntegerExpr{val})
		case ruleNil:
			tokens.Push(NilExpr{})
		case ruleUndefined:
			tokens.Push(UndefinedExpr{})
		case ruleCreateMap:
			tokens.Push(CreateMapExpr{})
		case ruleAssignment:
			rhs := tokens.Pop()
			lhs := tokens.Pop()
			m := tokens.Pop().(CreateMapExpr)
			m.Assignments = append(m.Assignments, Assignment{lhs, rhs})
			tokens.Push(m)

		case ruleBoolean:
			tokens.Push(BooleanExpr{contents == "true"})
		case ruleString:
			val := strings.Replace(contents[1:len(contents)-1], `\"`, `"`, -1)
			tokens.Push(StringExpr{val})
		case ruleIP:
			tokens.Push(StringExpr{contents})
		case ruleSubstitution:
			tokens.Push(SubstitutionExpr{Template: tokens.Pop()})

		case ruleConditional:
			fhs := tokens.Pop()
			ths := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(CondExpr{C: lhs, T: ths, F: fhs})

		case ruleLogOr:
			rhs := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(LogOrExpr{A: lhs, B: rhs})

		case ruleLogAnd:
			rhs := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(LogAndExpr{A: lhs, B: rhs})

		case ruleOr:
			rhs := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(OrExpr{A: lhs, B: rhs})

		case ruleNot:
			tokens.Push(NotExpr{tokens.Pop()})

		case ruleCompareOp:
			tokens.Push(operationHelper{op: contents})

		case ruleComparison:
			rhs := tokens.Pop()
			op := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(ComparisonExpr{A: lhs, Op: op.(operationHelper).op, B: rhs})

		case ruleConcatenation:
			rhs := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(ConcatenationExpr{A: lhs, B: rhs})
		case ruleAddition:
			rhs := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(AdditionExpr{A: lhs, B: rhs})
		case ruleSubtraction:
			rhs := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(SubtractionExpr{A: lhs, B: rhs})
		case ruleMultiplication:
			rhs := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(MultiplicationExpr{A: lhs, B: rhs})
		case ruleDivision:
			rhs := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(DivisionExpr{A: lhs, B: rhs})
		case ruleModulo:
			rhs := tokens.Pop()
			lhs := tokens.Pop()

			tokens.Push(ModuloExpr{A: lhs, B: rhs})

		case ruleName:
			tokens.Push(nameHelper{name: contents})
		case ruleNextName:
			rhs := tokens.Pop()
			list := tokens.PopNameList()
			list.list = append(list.list, rhs.(nameHelper).name)
			tokens.Push(list)

		case ruleMapping:
			rhs := tokens.Pop()
			lhs := tokens.Pop()
			tokens.Push(MapExpr{Lambda: rhs, A: lhs})

		case ruleSum:
			rhs := tokens.Pop()
			ini := tokens.Pop()
			lhs := tokens.Pop()
			tokens.Push(SumExpr{Lambda: rhs, A: lhs, I: ini})

		case ruleLambda:

		case ruleLambdaExpr:
			rhs := tokens.Pop()
			names := tokens.PopNameList().list
			tokens.Push(LambdaExpr{Names: names, E: rhs})

		case ruleLambdaRef:
			rhs := tokens.Pop()
			lexp, ok := rhs.(LambdaExpr)
			if ok {
				tokens.Push(lexp)
			} else {
				tokens.Push(LambdaRefExpr{Source: rhs, Path: path, StubPath: stubPath})
			}

		case ruleRange:
			rhs := tokens.Pop()
			lhs := tokens.Pop()
			tokens.Push(RangeExpr{lhs.(Expression), rhs.(Expression)})

		case ruleList:
			seq := tokens.PopExpressionList()
			tokens.Push(ListExpr{seq})

		case ruleNextExpression:
			rhs := tokens.Pop()
			list := tokens.Pop().(expressionListHelper)
			list.list = append(list.list, rhs)
			tokens.Push(list)

		case ruleStartList, ruleStartArguments:
			tokens.Push(expressionListHelper{})

		case ruleKey, ruleIndex:
		case ruleGrouped:
		case ruleLevel0, ruleLevel1, ruleLevel2, ruleLevel3, ruleLevel4, ruleLevel5, ruleLevel6, ruleLevel7:
		case ruleExpression:
		case ruleExpressionList:
		case ruleMap:
		case ruleAssignments:
		case rulews:
		case rulereq_ws:
		default:
			panic("unhandled:" + rul3s[token.pegRule])
		}
	}

	panic("unreachable")
}

func reverse(a []string) {
	for i := 0; i < len(a)/2; i++ {
		a[i], a[len(a)-i-1] = a[len(a)-i-1], a[i]
	}
}

func equals(p1 []string, p2 []string) bool {
	if len(p1) != len(p2) {
		return false
	}
	for i := 0; i < len(p1); i++ {
		if p1[i] != p2[i] {
			return false
		}
	}
	return true
}

type tokenStack struct {
	list.List
}

func (s *tokenStack) Pop() Expression {
	front := s.Front()
	if front == nil {
		return nil
	}

	s.Remove(front)

	return front.Value.(Expression)
}

func (s *tokenStack) Peek() Expression {
	front := s.Front()
	if front == nil {
		return nil
	}

	return front.Value.(Expression)
}

func (s *tokenStack) Push(expr Expression) {
	s.PushFront(expr)
}

func (s *tokenStack) PopExpressionList() []Expression {
	return (s.Pop().(expressionListHelper)).list
}

func (s *tokenStack) PopNameList() nameListHelper {
	lhs := s.Pop()
	list, ok := lhs.(nameListHelper)
	if !ok {
		list = nameListHelper{list: []string{lhs.(nameHelper).name}}
	}
	return list
}
