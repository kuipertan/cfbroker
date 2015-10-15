
package main

import (
	"github.com/kuipertan/types-cf"
)

var bullets = []string {"Auto banlance", "High available"}


var myplans = []*cf.Plan {
        {ID:"73dc6439-6bc8-43ea-a47a-cbe6abdf0e02",
         Name:"G1",
         Description:"1 GB capacity",
         Metadata:&cf.PlanMeta{Bullets:bullets, DisplayName:"Maxnum of 1 GB", Costs:"0"},
         Free: true},
        {ID:"63c60aef-ce88-4907-bff1-b60c9ae52b67",
         Name:"G2",
         Description:"2 GB capacity",                                                                                                                
         Metadata:&cf.PlanMeta{Bullets:bullets, DisplayName:"Maxnum of 2 GB", Costs:"0"},                                                            
         Free: true},                                                                                                                                
        {ID:"4b2c7904-0203-4b05-84f7-a4532eb18fe2",                                                                                                  
         Name:"G4",                                                                                                                                  
         Description:"4 GB capacity",                                                                                                                
         Metadata:&cf.PlanMeta{Bullets:bullets, DisplayName:"Maxnum of 4 GB", Costs:"0"},                                                            
         Free: true},                                                                                                                                
        {ID:"cbd9382b-ffea-4c5d-a487-bcfed09a42f2",                                                                                                  
         Name:"G8",                                                                                                                                  
         Description:"8 GB capacity",                                                                                                                
         Metadata:&cf.PlanMeta{Bullets:bullets, DisplayName:"Maxnum of 8 GB", Costs:"0"},                                                            
         Free: true},                                                                                                                                
}

