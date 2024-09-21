# Algoritmo di Chandy-Lamport
## Progetto SDCC - Snapshot di un sistema distribuito 
L'algoritmo di Chandy-Lamport, che prende il nome dai suoi inventori, ha lo scopo di eseguire uno snapshot globale di un sistema distribuito in maniera coerente. 

L'algoritmo funziona sotto determinate assunzioni:
- ogni processo registra solamente il proprio stato e quello dei canali in entrata;
- comunicazione affidabile;
- canali FIFO unidirezionali;
- l'esecuzione dell'algoritmo non influisce su quella dell'applicazione;
- ogni nodo può avviare la registrazione (anche più di uno).

## Funzionamento dell'algoritmo
Il processo che inizia l'algoritmo (uno o più):
1. registra il proprio stato
2. invia un messaggio di "marker" su tutti i suoi canali in uscita
3. inizia a registrare su tutti i suoi canali in entrata

Qunado un processo P_i riceve un messaggio di "marker" sul canale C_ki:
- se è il primo messaggio di "marker" che vede (inviato o ricevuto):
1. registra il proprio stato
2. marca il canale C_ki come vuoto
3. invia un messaggio di "marker" su tutti i suoi canali in uscita
4. inizia a registrare su tutti i canali in entrata, eccetto C_ki
- se ha già visto un messaggio di "marker":
1. smette di registrare su C_ki

Il messaggio di "marker" non contiene effettivamente nulla, ma serve solo ai fini dell'algoritmo.

## Esecuzione senza istanza EC2
Per l'esecuzione del codice senza un'ustanza EC2, è necessario prima clonare la repository da git:
```bash
git clone https://github.com/shenron221B/SDCC_project
```

e successivamente avviare i container tramite:
```bash
docker-compose -f compose.yaml up
```

## Istanza EC2
Per eseguire il codice tramite un'istanza EC2, occorre innanzitutto connettersi all'istanza tramite SSH sulla PowerShell:
```bash
ssh -i <path_to_pem> ec2-user@<ip_EC2_istance>
```

istallare Docker:
```bash
sudo yum update -y
sudo yum install -y docker
```

istallare Docker Compose:
```bash
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

eseguire il Docker deamon:
```bash
sudo service docekr start
```

istallare git e clonare la repository:
```bash
sudo yum install git -y
```

eseguire Docker Compose:
eseguire il Docker deamon:
```bash
cd SDCC_project
sudo docker-compose -f compose.yaml up
```

## Docker - comandi utili

Listare i container attivi:
eseguire il Docker deamon:
```bash
sudo docker ps
```

Stop di un container:
eseguire il Docker deamon:
```bash
sudo docker kill <container_ID>
```

Riavviare un container:
eseguire il Docker deamon:
```bash
sudo docker restart <container_ID>
```

Rimuovere tutte le immagini:
eseguire il Docker deamon:
```bash
sudo docker rmi $(sudo docker images -a -q)
```
