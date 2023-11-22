const botActionMillis = 1000;
const pingInterval = 1500;
const restartInterval = 2000;
const inactivityTimeoutMillis = 3000;

const PING = 'ping';
const GATE_CONNECT = 'gateconnect';
const GATE_ERROR = 'gateerror';
const GARAGE_CONNECT = 'garageconnect';
const GARAGE_ERROR = 'garageerror';
const GATE = 'gate:';
const GARAGE = 'garage:';

const params = new URLSearchParams(location.search);
const token = params.get('t');
const wsUrl = `ws://${location.host}/${token}/ws`;
const webrtcUrl = `${location.protocol}//${location.hostname}:8031/${token}`;

const video = document.getElementById('video');
//const cover = document.getElementById('cover');
const gateBtn = document.getElementById('gateBtn');
const garageBtn = document.getElementById('garageBtn');
const restartBtn = document.getElementById('restartBtn');

const events = new EventTarget();
var allStopped = false, focusLost = false;
var gateReady = false, garageReady = false;
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
        } else if(data == GARAGE_CONNECT) {
            events.dispatchEvent(new CustomEvent('garageconnect'));
        } else if(data.startsWith(GATE_ERROR)) {
            events.dispatchEvent(new CustomEvent('gateerror'));
        } else if(data.startsWith(GARAGE_ERROR)) {
            events.dispatchEvent(new CustomEvent('garageerror'));
        } else if(data.startsWith(GATE)) {
            events.dispatchEvent(new CustomEvent('botgate', { detail: { state: data.slice(GATE.length) } }));
        } else if(data.startsWith(GARAGE)) {
            events.dispatchEvent(new CustomEvent('botgarage', { detail: { state: data.slice(GARAGE.length) } }));
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
    video.pause();
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
    video.srcObject = new MediaStream([ detail.track ]);
    //cover.style.display = 'none';
});

events.addEventListener('webrtcsdp', ({ detail }) => {
    peerCon.setRemoteDescription(new RTCSessionDescription({
        type: 'answer',
        sdp: atob(detail.sdp)
    }));
});

events.addEventListener('gateconnect', () => {
    console.log('Gate connected');
    gateReady = true;
    gateBtn.classList.remove('inactive');
});
events.addEventListener('gateerror', () => {
    console.log('Failed connecting gate');
    gateBtn.classList.add('unavailable');
});
events.addEventListener('garageconnect', () => {
    console.log('Garage connected');
    garageReady = true;
    garageBtn.classList.remove('inactive');
});
events.addEventListener('garageerror', () => {
    console.log('Failed connecting garage');
    garageBtn.classList.add('unavailable');
});

restartBtn.onclick = () => location.reload();

gateBtn['ontouchstart' in gateBtn ? 'ontouchstart' : 'onmousedown'] = () => {
    if(!gateReady)
        return;
    console.log("Press gate");
    gateBtn.classList.add('inactive');
    ws.send('gate');
};
garageBtn['ontouchstart' in garageBtn ? 'ontouchstart' : 'onmousedown'] = () => {
    if(!garageReady)
        return;
    console.log("Press garage");
    garageBtn.classList.add('inactive');
    ws.send('garage');
};

events.addEventListener('botgate', ({ detail }) => {
    console.log("Gate " + detail.state);
    setTimeout(() => {
        gateBtn.classList.remove('inactive');
    }, botActionMillis);
});
events.addEventListener('botgarage', ({ detail }) => {
    console.log("Garage " + detail.state);
    setTimeout(() => {
        garageBtn.classList.remove('inactive');
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
