import json,subprocess,urllib.request,urllib.parse,datetime

def inspect(spec):
    now=datetime.datetime.now(datetime.timezone.utc)
    prefix=spec['run_id'][:8]
    result={}
    for machine in spec['machines']:
        agent=prefix+'-'+machine['name']
        query='"graphene.namespace":="t-stroppy-live" AND "graphene.agent":='+json.dumps(agent)+' AND "service.name":="graphene-agent" AND (_msg:="connected" OR _msg:="session ended, reconnecting")'
        req=urllib.request.Request('http://127.0.0.1:19428/select/logsql/query',data=urllib.parse.urlencode({'query':query,'limit':100}).encode())
        with urllib.request.urlopen(req,timeout=20) as response: rows=[json.loads(line) for line in response if line.strip()]
        rows.sort(key=lambda x:x['_time'])
        connected=[x for x in rows if x['_msg']=='connected']; errors=[x for x in rows if x['_msg']!='connected']
        state=json.loads(subprocess.check_output(['graphenectl','-n','t-stroppy-live','get','agent',agent,'--jq','.resource'],text=True,timeout=20))
        assert state['phase']=='ready' and state['state']['agentConnected'],(agent,'not connected')
        assert len(connected)==1 and not errors,(agent,'unexpected reconnect',len(connected),len(errors))
        first=datetime.datetime.fromisoformat(connected[0]['_time'].replace('Z','+00:00'));age=(now-first).total_seconds()
        assert age>120,(agent,'connection observation too short',age)
        result[agent]={'connected':True,'first_connected_at':first.isoformat(),'checked_at':now.isoformat(),'observed_connection_seconds':age,'connection_events':len(connected),'disconnect_events':len(errors)}
    return result
