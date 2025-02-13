package database

import (
	"database/sql"
	"fmt"

	"os"

	//Register the sqlite3 lib
	_ "github.com/mattn/go-sqlite3"
	"github.com/sharovik/devbot/internal/config"
	"github.com/sharovik/devbot/internal/dto"
	"github.com/sharovik/devbot/internal/helper"
)

//SQLiteDictionary the sqlite dictionary object
type SQLiteDictionary struct {
	client *sql.DB
	Cfg    config.Config
}

//InitDatabaseConnection initialise the database connection
func (d *SQLiteDictionary) InitDatabaseConnection() error {
	if _, err := os.Stat(d.Cfg.DatabaseHost); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", d.Cfg.DatabaseHost)
	if err != nil {
		return err
	}

	d.client = db
	return nil
}

//CloseDatabaseConnection method for database connection close
func (d *SQLiteDictionary) CloseDatabaseConnection() error {
	return d.client.Close()
}

//FindAnswer used for searching of message in the database
func (d SQLiteDictionary) FindAnswer(message *dto.SlackResponseEventMessage) (dto.DictionaryMessage, error) {
	var (
		dmAnswer dto.DictionaryMessage
		regexID  int64
		err      error
	)

	//We do that because it can be that we can parse this question by available regex. If so, it will help main query to find the answer for this message
	regexID, err = d.parsedByAvailableRegex(message.Text)
	if err != nil {
		return dto.DictionaryMessage{}, err
	}

	dmAnswer, err = d.answerByQuestionString(message.Text, regexID)
	if err != nil {
		return dto.DictionaryMessage{}, err
	}

	//Finally we parse data by using selected regex in our question
	if dmAnswer.Regex != "" {
		matches := helper.FindMatches(dmAnswer.Regex, message.Text)

		if len(matches) != 0 && dmAnswer.MainGroupIndexInRegex != "" && matches[dmAnswer.MainGroupIndexInRegex] != "" {
			dmAnswer.Answer = fmt.Sprintf(dmAnswer.Answer, matches[dmAnswer.MainGroupIndexInRegex])
		}
	}

	return dmAnswer, nil
}

func (d SQLiteDictionary) parsedByAvailableRegex(question string) (int64, error) {
	availableRegex, err := d.GetAllRegex()
	if err != nil {
		return int64(0), err
	}

	for regexID, regex := range availableRegex {
		matches := helper.FindMatches(regex, question)
		if len(matches) != 0 {
			return regexID, nil
		}
	}

	return 0, nil
}

//answerByQuestionString method retrieves the answer data by selected question string
func (d SQLiteDictionary) answerByQuestionString(questionText string, regexID int64) (dto.DictionaryMessage, error) {
	var (
		id                 int64
		answer             string
		question           string
		questionRegex      sql.NullString
		questionRegexGroup sql.NullString
		alias              string
		err                error
	)

	query := `
		select
		s.id,
		q.answer,
		q.question,
		qr.regex as question_regex,
		qr.regex_group as question_regex_group,
		e.alias
		from questions q
		join scenarios s on q.scenario_id = s.id
		left join questions_regex qr on qr.id = q.regex_id
		left join events e on s.event_id = e.id
	`

	if regexID != 0 {
		query = query + " where q.regex_id = ? order by e.id limit 1"
		err = d.client.QueryRow(query, regexID).Scan(&id, &answer, &question, &questionRegex, &questionRegexGroup, &alias)
	} else {
		query = query + " where q.question like ? order by e.id limit 1"
		err = d.client.QueryRow(query, questionText+"%").Scan(&id, &answer, &question, &questionRegex, &questionRegexGroup, &alias)
	}

	if err == sql.ErrNoRows {
		return dto.DictionaryMessage{}, nil
	} else if err != nil {
		return dto.DictionaryMessage{}, err
	}

	return dto.DictionaryMessage{
		ScenarioID:            id,
		Answer:                answer,
		Question:              question,
		Regex:                 questionRegex.String,
		MainGroupIndexInRegex: questionRegexGroup.String,
		ReactionType:          alias,
	}, nil
}

//InsertScenario used for scenario creation
func (d SQLiteDictionary) InsertScenario(name string, eventID int64) (int64, error) {
	result, err := d.client.Exec(`insert into scenarios (name, event_id) values ($1, $2)`, name, eventID)
	if err != nil {
		return 0, err
	}

	var scenarioID int64
	scenarioID, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return scenarioID, nil
}

//FindScenarioByID search scenario by id
func (d SQLiteDictionary) FindScenarioByID(scenarioID int64) (int64, error) {
	err := d.client.QueryRow("select id from scenarios where id = $1", scenarioID).Scan(&scenarioID)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return scenarioID, nil
}

//GetLastScenarioID retrieve the last scenario id
func (d SQLiteDictionary) GetLastScenarioID() (int64, error) {
	var scenarioID int64
	err := d.client.QueryRow("select id from scenarios order by id desc limit 1").Scan(&scenarioID)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return scenarioID, nil
}

//FindEventByAlias search event by alias
func (d SQLiteDictionary) FindEventByAlias(eventAlias string) (int64, error) {
	var eventID int64
	err := d.client.QueryRow("select id from events where alias = $1", eventAlias).Scan(&eventID)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return eventID, nil
}

//InsertEvent used for event creation
func (d SQLiteDictionary) InsertEvent(alias string) (int64, error) {
	result, err := d.client.Exec(`insert into events (alias) values ($1)`, alias)
	if err != nil {
		return 0, err
	}

	var lastInsertID int64
	lastInsertID, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return lastInsertID, nil
}

//InsertQuestion inserts the question into the database
func (d SQLiteDictionary) InsertQuestion(question string, answer string, scenarioID int64, questionRegex string, questionRegexGroup string) (int64, error) {
	var (
		regexID int64
		err     error
	)
	if questionRegex != "" {
		//We need to find the existing regexID
		regexID, err = d.FindRegex(questionRegex)
		if err != nil {
			return 0, err
		}

		//If we don't have this regex in our database, then we need to add it
		if regexID == 0 {
			regexID, err = d.InsertQuestionRegex(questionRegex, questionRegexGroup)
			if err != nil {
				return 0, err
			}
		}
	}

	result, err := d.client.Exec(`insert into questions (question, answer, scenario_id, regex_id) values ($1, $2, $3, $4)`,
		question,
		answer,
		scenarioID,
		regexID,
	)
	if err != nil {
		return 0, err
	}

	var questionID int64
	questionID, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return questionID, nil
}

//FindRegex search regex by regex string
func (d SQLiteDictionary) FindRegex(regex string) (int64, error) {
	var regexID int64
	err := d.client.QueryRow("select id from questions_regex where regex = $1", regex).Scan(&regexID)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return regexID, nil
}

//InsertQuestionRegex method insert the regex and returns the regexId. This regex can be connected to the multiple questions
func (d SQLiteDictionary) InsertQuestionRegex(questionRegex string, questionRegexGroup string) (int64, error) {
	result, err := d.client.Exec(`insert into questions_regex (regex, regex_group) values ($1, $2)`,
		questionRegex,
		questionRegexGroup,
	)
	if err != nil {
		return 0, err
	}

	var regexID int64
	regexID, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return regexID, nil
}

//GetAllRegex method retrieves all available regex from questions_regex
func (d SQLiteDictionary) GetAllRegex() (map[int64]string, error) {
	var result = map[int64]string{}
	rows, err := d.client.Query("select id, regex from questions_regex")
	if err == sql.ErrNoRows {
		return map[int64]string{}, nil
	} else if err != nil {
		return map[int64]string{}, err
	}

	var (
		id    int64
		regex string
	)

	for rows.Next() {
		if err := rows.Scan(&id, &regex); err != nil {
			return map[int64]string{}, err
		}

		result[id] = regex
	}

	return result, nil
}
