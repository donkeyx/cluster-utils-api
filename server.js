'use strict';

const express = require('express');
const os = require('os');
const morgan = require('morgan')

// Constants
const PORT = 80;
const HOST = '0.0.0.0';

const INFO = {
    version: 'v1.1',
    endpoints: [
        '/statsz',
        'healthz',
        '/ping',
        '/envz',
        '/readyness_delay'
    ]
}




// some basic endpoints to test readyness and zpages

const app = express();
app.use(morgan('combined'));

app.get('/', (req, res) => {
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


app.listen(PORT, HOST);
console.log(`Running on http://${HOST}:${PORT}`);

