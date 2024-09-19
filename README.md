Realtimer

Is a service that turns SQL database operations 
to events that can be subscribed/listened to by client
It is aimed to support MySQL and POSTGRESS databases


service will initialize and manage database TRIGGERs
service will run continously awaiting TRIGGER invoke 
and emitting events

sercice will using config file to specify
runtime config like db cred and table events

Components needed
- Adaptar: connect to MySQL/Postgress db and runs querys
- API: expose endpoints to listen to db invokations
- WSS: ws server to emit events
- Config file parser

Workflow: parse config -> connect to db -> create/verfiy triggers -> listen to changes

build udf
 - gcc $(dir of mysql.h) -shared -fPIC -o http_request.so http_request.c
