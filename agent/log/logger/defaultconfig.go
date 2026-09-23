package logger

func DefaultConfig() []byte {
	return LoadLog(DefaultLogDir, LogFile, "info")
}

func LoadLog(defaultLogDir string, logFile string, debugStatus string) []byte {
	logConfig := `
<seelog type="adaptive" mininterval="2000000" maxinterval="100000000" critmsgcount="500" minlevel="` + debugStatus + `">
    <exceptions>
        <exception filepattern="test*" minlevel="error"/>
    </exceptions>
    <outputs formatid="fmtinfo">
        <console formatid="fmtinfo"/>
    </outputs>
    <formats>
        <format id="fmterror" format="%Date(2006-01-02 15:04:05.0000) %LEVEL [%FuncShort @ %File.%Line] %Msg%n"/>
        <format id="fmtdebug" format="%Date(2006-01-02 15:04:05.0000) %LEVEL [%FuncShort @ %File.%Line] %Msg%n"/>
        <format id="fmtinfo" format="%Date(2006-01-02 15:04:05.0000) %LEVEL %Msg%n"/>
    </formats>
</seelog>
`
	return []byte(logConfig)
}
