async function register() {
    console.log("registering");
    
    const fetch_prom = fetch("http://127.0.0.1:80/api/register", {
        method: "POST",
        body: host.aurl + "," + host.idCert + "," + host.hsKeyCert,
    });
    console.log("sent fetch")

    const response = await fetch_prom;
    console.log("received response!");
    
    if (response.status !== 200) {
        console.log("failed to register: " 
            + (await response.text()).trim());
        return;
    }
    console.log(await response.text());

    while(true) {
        console.log(".");
        const response = await fetch("http://127.0.0.1:80/api/wait?id=" + host.id);
        if (response.status === 408) { //timeout, no one requested to join.
            continue; //waiting
        } else if (response.status === 200) {
            // received randezvous request
            const bodyParts = body.split(",", 3);
            if (bodyParts.length != 3) {
                console.log("failed to parse response: (" + len(bodyParts) + ")");
                continue;
            }
            console.log(bodyParts[0] + " wants to connect me.")

            host.register(bodyParts[1], bodyParts[2]);
            console.log("(main.aml)registered peer " + bodyParts[0]);

            await sleep(100);

            console.log("(main.aml)connecting peer " + bodyParts[0]);
            host.connect(bodyParts[0]);
        } else {
            console.log("failed to wait for event: " 
                + (await response.text()));
            return;
        }
    }
}
register();