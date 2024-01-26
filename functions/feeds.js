const express = require("express"); 
const serverless = require('serverless-http');
const xml = require('xml');
const axios = require('axios');
const { error } = require("console");
  
// Create Express Server 
const app = express();

app.get("/er_waittime", (request, response) => {
    response.send("Hi there, please pass the facility code...");
});

app.get("/er_waittime/:faccodes", (request, response) => {
    let { ERURL , ERTOKEN } = process.env; 
    let FACILITY_CODE = request.params.faccodes; 
    let API_SERVICE_URL = `${ERURL}?faccodes=${FACILITY_CODE}`;
    let config = {
        headers: { Authorization: `Bearer ${ERTOKEN}` }
    };

    axios.get( 
        API_SERVICE_URL,
        config
    ).then(function(result){

        if(Array.isArray(result.data)){
            
            procesMultipleErWaitTimes(result.data).then((feedObject) => {
                console.log(feedObject);
                const feed = '<?xml version="1.0" encoding="UTF-8"?>' + xml(feedObject);

                response.setHeader('Content-Type', 'text/xml');
                // response.setHeader('Content-Disposition', `attachment; filename=${FACILITY_CODE}.rss`);

                response.send(feed);
            });
            
        }
        
    }).catch(function (error){
        console.log(error);
        response.status(403).render();
    });
});

async function procesMultipleErWaitTimes (data) {

    let feedObject = {
        rss: [
            {
                _attr: {
                version: "2.0",
                "xmlns:atom": "http://www.w3.org/1999/xhtml",
                name: "robots",
                content: "noindex"
                },
            },
            {
                channel: [
                    {
                        title: "ER Wait Time",
                    },
                    { description: "RSS feed for ER wait time" }
                ],
            },
        ],
    };

    await Promise.all(data.map(async (erTime) =>
    { 
        let { facilityCode, waitTimeInSeconds } = erTime;
        let minutes = Math.floor(waitTimeInSeconds / 60);

        let item = [
            { title: `Hospital Code ${facilityCode}` },
            { description: `${minutes} Minutes` },
            { time: minutes }
        ];

        feedObject.rss[1].channel.push({item: item});
    }));
    return feedObject;
}
  
// Starting our Proxy server 
// app.listen(PORT, HOST, () => { 
//     console.log(`Starting Server at ${HOST}:${PORT}`); 
// }); 

module.exports.handler = serverless(app);