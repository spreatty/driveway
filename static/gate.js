const botActionMillis = 1000;
const pingInterval = 1500;
const restartInterval = 2000;
const inactivityTimeoutMillis = 3000;

const PING = 'ping';
const GATE_CONNECT = 'gateconnect';
const GATE_ERROR = 'gateerror';
const GATE = 'gate:';

const params = new URLSearchParams(location.search);
const token = params.get('t');
const wsUrl = `ws://${location.host}/${token}/mono`;
const webrtcUrl = `${location.protocol}//${location.hostname}:8031/${token}`;

const gateVideo = document.getElementById('gate-video');
const gateBtn = document.getElementById('gate-frame');
const gateState = document.getElementById('gate-state');

const events = new EventTarget();
var allStopped = false, focusLost = false;
var gateReady = false;
var pingTimeout, blurTimeout, restartTimeout;
var peerCon;
var ws;

const _log = console.log.bind(console);
console.log = (...args) => {
    _log.apply(null, [new Date().toISOString().slice(11, -1)].concat(args))
};

connectWebSocket();
connectWebRTC();

function connectWebSocket() {
    console.log(`Connecting to ${wsUrl}`);

    ws = new WebSocket(wsUrl);

    ws.onerror = () => {
        events.dispatchEvent(new CustomEvent('wserror'));
    };

    ws.onopen = () => {
        events.dispatchEvent(new CustomEvent('wsopen'));
    };

    ws.onmessage = ({ data }) => {
        if(data == PING) {
            events.dispatchEvent(new CustomEvent('ping'));
        } else if(data == GATE_CONNECT) {
            events.dispatchEvent(new CustomEvent('gateconnect'));
        } else if(data.startsWith(GATE_ERROR)) {
            events.dispatchEvent(new CustomEvent('gateerror'));
        } else if(data.startsWith(GATE)) {
            events.dispatchEvent(new CustomEvent('botgate', { detail: { state: data.slice(GATE.length) } }));
        }
    };
}

function connectWebRTC() {
    peerCon = new RTCPeerConnection();
    peerCon.onnegotiationneeded = () => events.dispatchEvent(new CustomEvent('webrtcnegotiate'));
    peerCon.ontrack = ({ track }) => events.dispatchEvent(new CustomEvent('webrtctrack', { detail: { track } }));
    peerCon.addTransceiver('video', { 'direction': 'sendrecv' });
}

function startTimeout() {
    pingTimeout = setTimeout(() => events.dispatchEvent(new CustomEvent('lostconnection')), pingInterval);
}

events.addEventListener('wsopen', () => {
    console.log('WebSocket connected');
    if(window.AADriveway)
        window.AADriveway.disableWifi();
    startTimeout();
});

events.addEventListener('ping', () =>{
    clearTimeout(pingTimeout);
    startTimeout();
});

function stopAll() {
    allStopped = true;
    clearTimeout(pingTimeout);
    clearTimeout(blurTimeout);
    gateVideo.pause();
    gateState.classList.add('inactive');
    if(peerCon) {
        peerCon.close();
    }
    ws.close();
}

function restart() {
    console.log('Restarting');
    allStopped = false;
    connectWebSocket();
    connectWebRTC();
}

function onlostconnection() {
    console.log('Lost connection');
    stopAll();
    if(focusLost) {
        return;
    }
    console.log('Scheduling restart');
    restartTimeout = setTimeout(restart, restartInterval);
}
events.addEventListener('lostconnection', onlostconnection);
events.addEventListener('wserror', onlostconnection);

events.addEventListener('webrtcnegotiate', () => {
    peerCon.createOffer()
        .then(offer => peerCon.setLocalDescription(offer))
        .then(() => fetch(webrtcUrl, {
            method: 'POST',
            mode: 'cors',
            cache: "no-cache",
            headers: {
              'content-type': 'application/x-www-form-urlencoded'
            },
            body: `data=${encodeURIComponent(btoa(peerCon.localDescription.sdp))}`
        }))
        .then(res => res.text())
        .then(data => events.dispatchEvent(new CustomEvent('webrtcsdp', { detail: { sdp: data } })));
});

events.addEventListener('webrtctrack', ({ detail }) => {
    gateVideo.srcObject = new MediaStream([ detail.track ]);
});

events.addEventListener('webrtcsdp', ({ detail }) => {
    var sdp = atob(detail.sdp);
    const sdpLines = sdp.split('\r\n');
    const candidatesStart = sdpLines.findIndex(l => l.startsWith('a=candidate'));
    const candidatesEnd = sdpLines.findIndex(l => l == 'a=end-of-candidates');
    var newSdpLines = sdpLines.slice(0, candidatesStart);
    const candidatesLines = sdpLines.slice(candidatesStart, candidatesEnd)
        .filter(l => l.endsWith('host'))
        .slice(0, 2)
        .map(l => l.replace(/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/, location.hostname));
    newSdpLines = newSdpLines.concat(candidatesLines, sdpLines.slice(candidatesEnd));
    sdp = newSdpLines.join('\r\n');
    console.log('SDP', sdp);
    peerCon.setRemoteDescription(new RTCSessionDescription({
        type: 'answer',
        sdp: sdp
    }));
});

events.addEventListener('gateconnect', () => {
    console.log('Gate connected');
    gateReady = true;
    gateState.classList.remove('inactive');
});
events.addEventListener('gateerror', () => {
    console.log('Failed connecting gate');
    gateState.classList.add('unavailable');
});

const reactEvent = window.AADriveway ? 'ontouchstart' : 'ontouched' in gateBtn ? 'ontouched' : 'onclick';
gateBtn[reactEvent] = () => {
    if(!gateReady)
        return;
    console.log("Press gate");
    gateState.classList.add('inactive');
    ws.send('gate');
};

events.addEventListener('botgate', ({ detail }) => {
    console.log("Gate " + detail.state);
    setTimeout(() => {
        gateState.classList.remove('inactive');
    }, botActionMillis);
});

window.onblur = () => {
    console.log('Focus lost');
    focusLost = true;
    ws.send('blur');
    clearTimeout(restartTimeout);
    blurTimeout = setTimeout(() => {
        console.log('Inactivity detected');
        ws.send('inactive');
        stopAll();
    }, inactivityTimeoutMillis);
};
window.onfocus = () => {
    console.log('Focus gained');
    ws.send('focus');
    if(!focusLost) {
        return;
    }
    
    focusLost = false;
    if(allStopped) {
        restart();
    } else {
        clearTimeout(blurTimeout);
    }
};
