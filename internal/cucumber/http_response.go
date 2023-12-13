// Assert response code is correct:
//
//	Then the response code should be 202
//
// Assert that a json field of the response body is correct.  This uses a http://github.com/itchyny/gojq expression to select the json field of the
// response:
//
//	Then the ".status" selection from the response should match "assigning"
//
// Assert that the response body matches the provided text:
//
//	Then the response should match "Hello"
//	Then the response should match:
//	"""
//	Hello
//	"""
//
// Assert that response json matches the provided json.  Differences in json formatting and field order are ignored.:
//
//	Then the response should match json:
//	  """
//	  {
//	      "id": "${cid}",
//	  }
//	  """
//
// Stores a json field of the response body in a scenario variable:
//
//	Given I store the ".id" selection from the response as ${cid}
//
// Stores a json value in a scenario variable:
//
//	Given I store json as ${$input}:
//	  """
//	  {
//	      "id": "${cid}",
//	  }
//	  """
//
// Assert that a response header matches the provided text:
//
// Then the response header "Content-Type" should match "application/json;stream=watch"
//
// Assert that a json field of the response body is correct matches the provided json:
//
//	Then the ".deployment_location" selection from the response should match json:
//	  """
//	  {
//	      "namespace_id": "default"
//	  }
//	  """
package cucumber

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/cucumber/godog"
	"github.com/itchyny/gojq"
	"github.com/pmezard/go-difflib/difflib"
)

func init() {
	StepModules = append(StepModules, func(ctx *godog.ScenarioContext, s *TestScenario) {
		ctx.Step(`^the response code should be (\d+)$`, s.theResponseCodeShouldBe)
		ctx.Step(`^the response should match json:$`, s.TheResponseShouldMatchJsonDoc)
		ctx.Step(`^the response should contain json:$`, s.TheResponseShouldContainJsonDoc)
		ctx.Step(`^the response should match:$`, s.theResponseShouldMatchText)
		ctx.Step(`^the response should match "([^"]*)"$`, s.theResponseShouldMatchText)
		ctx.Step(`^I store the "([^"]*)" selection from the response as \${([^"]*)}$`, s.iStoreTheSelectionFromTheResponseAs)
		ctx.Step(`^I store the \${([^"]*)} as \${([^"]*)}$`, s.iStoreVariableAsVariable)
		ctx.Step(`^I store the \${([^"]*)} as \${([^"]*)}$`, s.iStoreVariableAsVariable)
		ctx.Step(`^I delete the \${([^"]*)} "([^"]*)" key$`, s.iDeleteTheMapKey)
		ctx.Step(`^the "(.*)" selection from the response should match "([^"]*)"$`, s.theSelectionFromTheResponseShouldMatch)
		ctx.Step(`^the response header "([^"]*)" should match "([^"]*)"$`, s.theResponseHeaderShouldMatch)
		ctx.Step(`^the response header "([^"]*)" should start with "([^"]*)"$`, s.theResponseHeaderShouldStartWith)
		ctx.Step(`^the "([^"]*)" selection from the response should match json:$`, s.theSelectionFromTheResponseShouldMatchJson)
		ctx.Step(`^\${([^"]*)} is not empty$`, s.vpc_idIsNotEmpty)
		ctx.Step(`^"([^"]*)" should match "([^"]*)"$`, s.textShouldMatchText)
		ctx.Step(`^\${([^"]*)} should match:$`, s.theVariableShouldMatchText)
		ctx.Step(`^\${([^"]*)} should match yaml:$`, s.theVariableShouldMatchYaml)
		ctx.Step(`^\${([^"]*)} should contain json:$`, s.theVariableShouldContainJson)
	})
}

func (s *TestScenario) vpc_idIsNotEmpty(name string) error {
	value, err := s.Resolve(name)
	if err != nil {
		return err
	}
	if value == nil || value == "" {
		return fmt.Errorf("variable ${%s} is empty", name)
	}
	return nil
}

func (s *TestScenario) theResponseCodeShouldBe(expected int) error {
	session := s.Session()
	actual := session.Resp.StatusCode
	if expected != actual {
		return fmt.Errorf("expected response code to be: %d, but actual is: %d, body: %s", expected, actual, string(session.RespBytes))
	}
	return nil
}

func (s *TestScenario) TheResponseShouldMatchJsonDoc(expected *godog.DocString) error {
	return s.theResponseShouldMatchJson(expected.Content)
}
func (s *TestScenario) theResponseShouldMatchJson(expected string) error {
	err := s.theResponseHeaderShouldStartWith("Content-Type", "application/json")
	if err != nil {
		return err
	}

	session := s.Session()
	if len(session.RespBytes) == 0 {
		return fmt.Errorf("got an empty response from server, expected a json body")
	}

	return s.JsonMustMatch(string(session.RespBytes), expected, true)
}

func (s *TestScenario) TheResponseShouldContainJsonDoc(expected *godog.DocString) error {
	return s.theResponseShouldContainJson(expected.Content)
}

func (s *TestScenario) theResponseShouldContainJson(expected string) error {
	err := s.theResponseHeaderShouldStartWith("Content-Type", "application/json")
	if err != nil {
		return err
	}
	session := s.Session()

	if len(session.RespBytes) == 0 {
		return fmt.Errorf("got an empty response from server, expected a json body")
	}

	return s.JsonMustContain(string(session.RespBytes), expected, true)
}

func (s *TestScenario) theVariableShouldContainJson(variableName, expected string) error {
	expanded, err := s.Expand(fmt.Sprintf("${%s}", variableName))
	if err != nil {
		return err
	}
	return s.JsonMustContain(expanded, expected, true)
}

func (s *TestScenario) theVariableShouldContainYaml(variableName, expected string) error {
	expanded, err := s.Expand(fmt.Sprintf("${%s}", variableName))
	if err != nil {
		return err
	}
	return s.JsonMustContain(expanded, expected, true)
}
func (s *TestScenario) textShouldMatchText(actual, expected string) error {

	expanded, err := s.Expand(expected)
	if err != nil {
		return err
	}

	actual, err = s.Expand(actual)
	if err != nil {
		return err
	}

	if expanded != actual {
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(expanded),
			B:        difflib.SplitLines(actual),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  1,
		})
		return fmt.Errorf("actual does not match expected, diff:\n%s", diff)
	}
	return nil
}

func (s *TestScenario) theVariableShouldMatchText(variable string, expected *godog.DocString) error {
	expanded, err := s.Expand(expected.Content)
	if err != nil {
		return err
	}

	actual, err := s.ResolveString(variable)
	if err != nil {
		return err
	}

	if expanded != actual {
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(expanded),
			B:        difflib.SplitLines(actual),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  1,
		})
		return fmt.Errorf("actual does not match expected, diff:\n%s", diff)
	}
	return nil
}

func (s *TestScenario) theVariableShouldMatchYaml(variable string, expected *godog.DocString) error {
	actual, err := s.ResolveString(variable)
	if err != nil {
		return err
	}
	return s.YamlMustMatch(actual, expected.Content, true)
}

func (s *TestScenario) theResponseShouldMatchText(expected string) error {
	session := s.Session()

	expanded, err := s.Expand(expected, "defs", "ref")
	if err != nil {
		return err
	}

	actual := string(session.RespBytes)
	if expanded != actual {
		diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(expanded),
			B:        difflib.SplitLines(actual),
			FromFile: "Expected",
			FromDate: "",
			ToFile:   "Actual",
			ToDate:   "",
			Context:  1,
		})
		return fmt.Errorf("actual does not match expected, diff:\n%s", diff)
	}
	return nil
}

func (s *TestScenario) theResponseHeaderShouldMatch(header, expected string) error {
	session := s.Session()
	expanded, err := s.Expand(expected)
	if err != nil {
		return err
	}

	actual := session.Resp.Header.Get(header)
	if expanded != actual {
		return fmt.Errorf("reponse header '%s' does not match expected: %v, actual: %v, body:\n%s", header, expanded, actual, string(session.RespBytes))
	}
	return nil
}
func (s *TestScenario) theResponseHeaderShouldStartWith(header, prefix string) error {
	session := s.Session()
	prefix, err := s.Expand(prefix)
	if err != nil {
		return err
	}

	actual := session.Resp.Header.Get(header)
	if !strings.HasPrefix(actual, prefix) {
		return fmt.Errorf("reponse header '%s' does not start with prefix: %v, actual: %v, body:\n%s", header, prefix, actual, string(session.RespBytes))
	}
	return nil
}

func (s *TestScenario) iStoreVariableAsVariable(name string, as string) error {
	value, err := s.Resolve(name)
	if err != nil {
		return err
	}
	s.Variables[as] = value
	return nil
}

func (s *TestScenario) iDeleteTheMapKey(mapName string, key string) error {
	value, err := s.Resolve(mapName)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Map {
		return fmt.Errorf("variable %s is not a map", mapName)
	}

	v.SetMapIndex(reflect.ValueOf(key), reflect.Value{})
	return nil
}

func (s *TestScenario) iStoreTheSelectionFromTheResponseAs(selector string, as string) error {

	session := s.Session()
	doc, err := session.RespJson()
	if err != nil {
		return err
	}

	query, err := gojq.Parse(selector)
	if err != nil {
		return err
	}

	iter := query.Run(doc)
	if next, found := iter.Next(); found {
		s.Variables[as] = next
		return nil
	}
	return fmt.Errorf("expected JSON does not have node that matches selector: %s", selector)
}

func (s *TestScenario) theSelectionFromTheResponseShouldMatch(selector string, expected string) error {
	session := s.Session()
	doc, err := session.RespJson()
	if err != nil {
		return err
	}

	query, err := gojq.Parse(selector)
	if err != nil {
		return err
	}

	expected, err = s.Expand(expected, "defs", "ref")
	if err != nil {
		return err
	}

	iter := query.Run(doc)
	if actual, found := iter.Next(); found {
		if actual == nil {
			actual = "null" // use null to represent missing value
		} else {
			actual = fmt.Sprintf("%v", actual)
		}
		if actual != expected {
			return fmt.Errorf("selected JSON does not match. expected: %v, actual: %v", expected, actual)
		}
		return nil
	}
	return fmt.Errorf("expected JSON does not have node that matches selector: %s", selector)
}

func (s *TestScenario) theSelectionFromTheResponseShouldMatchJson(selector string, expected *godog.DocString) error {

	session := s.Session()
	doc, err := session.RespJson()
	if err != nil {
		return err
	}

	query, err := gojq.Parse(selector)
	if err != nil {
		return err
	}

	iter := query.Run(doc)
	if actual, found := iter.Next(); found {
		actual, err := json.Marshal(actual)
		if err != nil {
			return err
		}

		return s.JsonMustMatch(string(actual), expected.Content, true)
	}
	return fmt.Errorf("expected JSON does not have node that matches selector: %s", selector)
}
