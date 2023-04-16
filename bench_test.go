package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
)

type RequestInfo struct {
	Method  string
	Path    string
	Headers map[string]string
}

var program *vm.Program

func generateGroupByKeyExpr(info *RequestInfo) (string, error) {
	//program, err := expr.Compile("Method + Path")
	//if err != nil {
	//	return "", err
	//}
	output, err := expr.Run(program, info)
	if err != nil {
		return "", err
	}
	return output.(string), nil
}

func generateGroupByKeySwitch(info *RequestInfo) string {
	switch info.Method {
	case "GET":
		return "get_" + info.Path
	case "POST":
		return "post_" + info.Path
	case "PUT":
		return "put_" + info.Path
	case "DELETE":
		return "delete_" + info.Path
	default:
		return "other_" + info.Path
	}
}

func BenchmarkGroupByKeySwitch(b *testing.B) {
	requestInfo := RequestInfo{
		Method: "GET",
		Path:   "/api/v1/users",
	}

	for n := 0; n < b.N; n++ {
		generateGroupByKeySwitch(&requestInfo)
	}
}

func BenchmarkGroupByKeyExpr(b *testing.B) {
	requestInfo := RequestInfo{
		Method: "GET",
		Path:   "/api/v1/users",
	}

	program, err := expr.Compile("Method + Path")
	if err != nil {
		b.Error(err)
		return
	}

	for n := 0; n < b.N; n++ {
		output, err := expr.Run(program, &requestInfo)
		if err != nil {
			b.Error(err)
			return
		}
		_ = output.(string)
	}
}

func main() {
	requestInfo := RequestInfo{
		Method: "GET",
		Path:   "/api/v1/users",
	}

	groupByKey, err := generateGroupByKeyExpr(&requestInfo)
	if err != nil {
		fmt.Println("Error generating group by key:", err)
	} else {
		fmt.Println("Group by key using expr:", groupByKey)
	}

	groupByKey = generateGroupByKeySwitch(&requestInfo)
	fmt.Println("Group by key using switch case:", groupByKey)

	// Run the benchmarks
	fmt.Println("Running benchmarks...")
	time.Sleep(2 * time.Second)
	fmt.Println("Benchmarks for generateGroupByKeySwitch:")
	for i := 0; i < 3; i++ {
		result := testing.Benchmark(BenchmarkGroupByKeySwitch)
		//fmt.Printf("%s\t%s\n", result.Name(), result.String())
    fmt.Printf("%s\n",result)
	}
	fmt.Println("Benchmarks for generateGroupByKeyExpr:")
	program, err = expr.Compile("Method + Path")
	for i := 0; i < 3; i++ {
		result := testing.Benchmark(BenchmarkGroupByKeyExpr)
    fmt.Printf("%s\n",result)
		//fmt.Printf("%s\t%s\n", result.Name(), result.String())
	}
}

