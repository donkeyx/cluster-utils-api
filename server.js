'use strict';

const os = require('os');
const express = require('express');
const app = express();


// Constants
console.log("the passed port:", process.env.PORT)
console.log(process.env.PORT);
const PORT = process.env.PORT || 8080;
console.log("port after ", PORT)
const HOST = '0.0.0.0';
const INFO = {
    version: 'v1.1',
    endpoints: [
        '/statsz',
        '/healthz',
        '/ping',
        '/envz',
        '/readyness_delay'
    ]
}

// some basic endpoints to test readyness and zpages


// simple middleware to clean the url crap i.e /something/sddssdf/healthz to /healthz
app.use(require('express-bunyan-logger')());
app.use(function (req, res, next) {

    // skip root path
    if (req.url !== "/") {
        let cleanUrl = req.url;

        if (cleanUrl.endsWith('/')) {
            console.log("stripping trailing slash");
            cleanUrl = cleanUrl.replace(/\/$/, "")
        }

        // mutate to strip off prefix junk i.e /v1/test/healthz
        cleanUrl = "/" + /.*\/(.*)/.exec(cleanUrl)[1];
        req.log.info({ routing: "middleware", source_url: req.url, dest_url: cleanUrl });
        req.url = cleanUrl;
    }

    next();
});

app.get("/", (req, res) => {
    res.send(INFO);
});

app.get('/infoz', (req, res) => {
    res.send(INFO);
});

app.get('/ping', (req, res) => {
    res.send({ ping: 'pong' });
});

app.get('/healthz', (req, res) => {
    res.send({ status: 'UP' });
});

app.get('/statsz', (req, res) => {

    res.send({
        cpu: os.cpus(),
        totalmem: os.totalmem(),
        freemem: os.freemem(),
        loadaverage: os.loadavg()
    });
});

app.get('/envz', (req, res) => {
    res.send(process.env);
});

app.get('/readyness_delay', (req, res) => {
    setTimeout(function () {
        res.send({ sleep: '3000' });
    }, 3000);

});

// force dodgy routes back to ./infoz
app.get('*', function (req, res) {
    let redirectPath = (req.originalUrl + '/infoz').replace('//', '/');
    res.redirect(302, redirectPath);
});

app.listen(PORT, HOST);
console.log(`Running on http://${HOST}:${PORT}`);

