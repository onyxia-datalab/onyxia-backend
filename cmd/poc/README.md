# We tested a new implem of the service install workflow 


The main issue of the current implementation is that the my-lab/app put is sync. If the instal is long it's leads to a 503 error without any feedback to the user. 

Then, now the front request install and then make get request each 1000ms in order to know when install is ready. [cf](https://github.com/InseeFrLab/onyxia/blob/7e50e08028dc60ca179cd7c3b184252148e8a456/web/src/core/adapters/onyxiaApi/onyxiaApi.ts#L607~L635). 


I propose to switch to an stream event conception where the front request install, and then listen a stream to know status whitout block the thread and request lots of time the api kubernetes. 


There is 2 options, using helm wait option and configure waiters so helm sdk make request and the other one is using kube informers that listen kube event whitout needed to get the api server. 


