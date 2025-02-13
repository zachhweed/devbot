package dictionary

import (
	"fmt"
	"html"
	"strconv"

	"github.com/sharovik/devbot/internal/container"
	"github.com/sharovik/devbot/internal/dto"
	"github.com/sharovik/devbot/internal/helper"
)

const (
	//EventName the name of the event
	EventName = "dictionary"

	//Regex for catching of the information from the received message
	regexScenarioIDAttribute         = "(?im)((?:scenario id:) (?P<scenario_id>.+))"
	regexScenarioNameAttribute       = "(?im)((?:scenario name:) (?P<scenario_name>.+))"
	regexQuestionAttribute           = "(?im)((?:question:) (?P<question>.+))"
	regexQuestionRegexAttribute      = "(?im)((?:question regex:) (?P<question_regex>.+))"
	regexQuestionRegexGroupAttribute = "(?im)((?:question regex group:) (?P<question_regex_group>.+))"
	regexAnswerAttribute             = "(?im)((?:answer:) (?P<answer>.+))"
	regexEventAliasAttribute         = "(?im)((?:event alias:) (?P<event_alias>.+))"
)

var (
	scenarioID         int64
	scenarioName       string
	question           string
	questionRegex      string
	questionRegexGroup string
	answer             string
	eventAlias         string
)

//ThemerEvent the struct for the event object
type ThemerEvent struct {
	EventName string
}

//Event - object which is ready to use
var Event = ThemerEvent{
	EventName: EventName,
}

//Execute method which is called by message processor
func (e ThemerEvent) Execute(message dto.SlackRequestChatPostMessage) (dto.SlackRequestChatPostMessage, error) {
	var answerMessage = message

	if err := parseAttributes(html.UnescapeString(message.OriginalMessage.Text)); err != nil {
		answerMessage.Text = "Error received during the attributes parsing: " + err.Error()
		return answerMessage, err
	}

	//We get the event id for selected event alias. The eventID we will use for scenarioID and question inserting
	eventID, err := container.C.Dictionary.FindEventByAlias(eventAlias)
	if err != nil {
		panic(err)
	}

	//If we received empty event id, it means that for that event-alias we don't have any row created. We need to create it now
	if eventID == 0 {
		eventID, err = container.C.Dictionary.InsertEvent(eventAlias)
		if err != nil {
			panic(err)
		}
	}

	//Now we need to do the similar procedure for the scenarioID
	scenarioID, err = container.C.Dictionary.FindScenarioByID(scenarioID)
	if err != nil {
		panic(err)
	}

	//If the scenarioID is 0 it means that scenarioID is not created. We need to create it now
	if scenarioID == 0 {
		lastScenarioID, err := container.C.Dictionary.GetLastScenarioID()
		if err != nil {
			panic(err)
		}

		if scenarioName == "" {
			scenarioName = fmt.Sprintf("Scenario #%d", lastScenarioID+1)
		}

		scenarioID, err = container.C.Dictionary.InsertScenario(scenarioName, eventID)
		if err != nil {
			panic(err)
		}
	}

	//In that step we have valid scenarioID and eventID. It means that we can proceed with question creation
	var questionID int64
	questionID, err = container.C.Dictionary.InsertQuestion(question, answer, scenarioID, questionRegex, questionRegexGroup)
	if err != nil {
		panic(err)
	}

	answerMessage.Text = fmt.Sprintf("I added this information to the dictionary.\nQuestionID: %d\nQuestion: %s\nAnswer: %s\nScenarioID: %d\nRegex: %s\nRegex group: %s", questionID, question, answer, scenarioID, questionRegex, questionRegexGroup)
	return answerMessage, nil
}

func parseAttributes(text string) error {
	var (
		err                 error
		_scenarioID         string
		_scenarioName       string
		_question           string
		_questionRegex      string
		_questionRegexGroup string
		_answer             string
		_eventAlias         string
	)

	_scenarioID = parseAttribute(text, regexScenarioIDAttribute, "scenario_id")
	_scenarioName = parseAttribute(text, regexScenarioNameAttribute, "scenario_name")
	_question = parseAttribute(text, regexQuestionAttribute, "question")
	_questionRegex = parseAttribute(text, regexQuestionRegexAttribute, "question_regex")
	_questionRegexGroup = parseAttribute(text, regexQuestionRegexGroupAttribute, "question_regex_group")
	_answer = parseAttribute(text, regexAnswerAttribute, "answer")
	_eventAlias = parseAttribute(text, regexEventAliasAttribute, "event_alias")

	if _question == "" {
		return fmt.Errorf("Question cannot be empty. ")
	}

	if _answer == "" {
		return fmt.Errorf("Question cannot be empty. ")
	}

	if _eventAlias == "" {
		return fmt.Errorf("Question cannot be empty. ")
	}

	if _scenarioID == "" {
		scenarioID = int64(0)
	} else {
		scenarioID, err = strconv.ParseInt(_scenarioID, 10, 64)
		if err != nil {
			return err
		}
	}

	scenarioName = _scenarioName
	question = _question
	questionRegex = _questionRegex
	questionRegexGroup = _questionRegexGroup
	answer = _answer
	eventAlias = _eventAlias

	return nil
}

func parseAttribute(text string, regex string, group string) string {
	matches := helper.FindMatches(regex, text)

	if len(matches) != 0 && group != "" && matches[group] != "" {
		return matches[group]
	}

	return ""
}
