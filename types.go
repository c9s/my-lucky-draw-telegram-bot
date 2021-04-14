package main

type Config struct {
	Messages struct {
		LuckyDrawStart                             string `json:"luckyDrawStart" yaml:"luckyDrawStart"`
		WillChooseNumberOfPersons                  string `json:"willChooseNumberOfPersons" yaml:"willChooseNumberOfPersons"`
		WillChooseOnePerson                        string `json:"willChooseOnePerson" yaml:"willChooseOnePerson"`
		WinnerIs                                   string `json:"winnerIs" yaml:"winnerIs"`
		ThereAreNMembersJoined                     string `json:"thereAreNMembersJoined" yaml:"thereAreNMembersJoined"`
		ThereIsOneMemberJoined                     string `json:"thereIsOneMemberJoined" yaml:"thereIsOneMemberJoined"`
		NoOneJoined                                string `json:"noOneJoined" yaml:"noOneJoined"`
		TheDrawIsOver                              string `json:"theDrawIsOver" yaml:"theDrawIsOver"`
		TheDrawIsNotStartedYet                     string `json:"theDrawIsNotStartedYet" yaml:"theDrawIsNotStartedYet"`
		TheDrawIsAlreadyStartedAndHasNotStoppedYet string `json:"theDrawIsAlreadyStartedAndHasNotStoppedYet" yaml:"theDrawIsAlreadyStartedAndHasNotStoppedYet"`
		NotifyWinner                               string `json:"notifyWinner" yaml:"notifyWinner"`
		AllMembersGotTheirPrize                    string `json:"allMembersGotTheirPrize" yaml:"allMembersGotTheirPrize"`
	} `json:"messages" yaml:"messages"`
}
