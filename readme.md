
#### Usage

```
./gcloud-subnet-rangefinder -project my-project -network my-network -sizes 16,18,20,16
```

```sh
                 -- subnet/public-subnetwork (10.4.0.0/20)
                 -- subnet/private-subnetwork (10.4.16.0/20)
                 -- subnet/dev-subnetwork (10.4.32.0/20)
                 -- < new > (10.4.48.0/20)
             -- < new > (10.4.64.0/18)
                 -- secondary/public-services (10.5.0.0/20)
                 -- secondary/private-services (10.5.16.0/20)
       ---- < new > (10.6.0.0/16)
         -------------------------- clusters/gke-nodes (10.100.0.0/28)
         -- < new > (10.101.0.0/16)

```
