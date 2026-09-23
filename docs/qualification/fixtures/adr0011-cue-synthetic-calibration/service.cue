package calibration

#Service: {
	endpoint: #Endpoint
	retries:  int & >=0
}

service: #Service & {
	endpoint: {
		host: "localhost"
		port: 8080
	}
	retries: 0
}
