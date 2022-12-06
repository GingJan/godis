package src

func _serverAssert(estr string, file string, line int) {
	//bugReportStart()
	//serverLog(LL_WARNING,"=== ASSERTION FAILED ===");
	//serverLog(LL_WARNING,"==> %s:%d '%s' is not true",file,line,estr);

	//if (server.crashlog_enabled) {
	//#ifdef HAVE_BACKTRACE
	//logStackTrace(NULL, 1);
	//#endif
	//printCrashReport();
	//}

	//// remove the signal handler so on abort() we will output the crash report.
	//removeSignalHandlers();
	//bugReportEnd(0, 0);
}
